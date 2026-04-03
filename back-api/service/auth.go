package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
)

// AuthService handles authentication business logic.
type AuthService interface {
	Register(ctx context.Context, username, email, password string) (uuid.UUID, error)
	Login(ctx context.Context, email, password string) (*auth.TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (*auth.TokenPair, error)
	Logout(ctx context.Context, refreshToken, accessToken string, claims *auth.Claims) error
}

type authService struct {
	repo     repository.Store
	sessions auth.SessionService
	cfg      config.JWTConfig
}

func NewAuthService(repo repository.Store, sessions auth.SessionService, cfg config.JWTConfig) AuthService {
	return &authService{repo: repo, sessions: sessions, cfg: cfg}
}

const (
	maxUsernameLen = 100
	minPasswordLen = 10
	maxPasswordLen = 72 // bcrypt limit

	maxLoginAttempts   int64 = 10
	loginLockoutWindow       = 15 * time.Minute

	maxRefreshAttempts int64 = 5
	refreshWindow            = 5 * time.Minute
)

var passwordASCIIOnly = regexp.MustCompile(`^[\x21-\x7E]+$`)

func (s *authService) Register(ctx context.Context, username, email, password string) (uuid.UUID, error) {
	if username == "" || email == "" || password == "" {
		return uuid.Nil, fmt.Errorf("username, email, and password are required: %w", apperr.ErrValidation)
	}
	if len(username) > maxUsernameLen {
		return uuid.Nil, fmt.Errorf("username must not exceed %d characters: %w", maxUsernameLen, apperr.ErrValidation)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return uuid.Nil, fmt.Errorf("invalid email format: %w", apperr.ErrValidation)
	}
	if len(password) < minPasswordLen {
		return uuid.Nil, fmt.Errorf("password must be at least %d characters: %w", minPasswordLen, apperr.ErrValidation)
	}
	if len(password) > maxPasswordLen {
		return uuid.Nil, fmt.Errorf("password must not exceed %d bytes: %w", maxPasswordLen, apperr.ErrValidation)
	}
	if !passwordASCIIOnly.MatchString(password) {
		return uuid.Nil, fmt.Errorf("password must contain only English letters, digits, and symbols (no spaces): %w", apperr.ErrValidation)
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return uuid.Nil, apperr.WrapInternal("hash password", err)
	}

	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	}

	if err := s.repo.RunTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		if err := txRepo.CreateUser(ctx, user); err != nil {
			return err
		}
		return txRepo.AssignRoleToUser(ctx, user.ID, auth.RoleUser)
	}); err != nil {
		if apperr.IsConflict(err) || apperr.IsValidation(err) {
			return uuid.Nil, err
		}
		return uuid.Nil, apperr.WrapInternal("register user", err)
	}

	return user.ID, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (*auth.TokenPair, error) {
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required: %w", apperr.ErrValidation)
	}

	// Brute-force protection
	attempts, err := s.sessions.IncrLoginAttempts(ctx, email, loginLockoutWindow)
	if err != nil {
		slog.WarnContext(ctx, "Failed to check login attempts, allowing request", "email", email, "error", err)
	} else if attempts > maxLoginAttempts {
		return nil, ErrTooManyAttempts
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if apperr.IsNotFound(err) {
			return nil, fmt.Errorf("invalid email or password: %w", apperr.ErrInvalidCredentials)
		}
		return nil, apperr.WrapInternal("get user by email", err)
	}

	if !auth.CheckPassword(password, user.PasswordHash) {
		return nil, fmt.Errorf("invalid email or password: %w", apperr.ErrInvalidCredentials)
	}

	// Successful login — reset counter
	if err := s.sessions.ResetLoginAttempts(ctx, email); err != nil {
		slog.WarnContext(ctx, "Failed to reset login attempts", "email", email, "error", err)
	}

	return s.issueTokenPair(ctx, user)
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*auth.TokenPair, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh_token is required: %w", apperr.ErrValidation)
	}

	tokenHash := auth.HashToken(refreshToken)

	// Rate-limit refresh attempts
	refreshKey := "refresh:" + tokenHash
	attempts, err := s.sessions.IncrLoginAttempts(ctx, refreshKey, refreshWindow)
	if err != nil {
		slog.WarnContext(ctx, "Failed to check refresh attempts, allowing request", "error", err)
	} else if attempts > maxRefreshAttempts {
		return nil, ErrTooManyAttempts
	}

	userID, err := s.sessions.GetRefreshTokenUserID(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", apperr.ErrInvalidToken)
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", apperr.ErrInvalidToken)
	}

	user, err := s.repo.GetUserByID(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", apperr.ErrInvalidToken)
	}

	// Issue new pair FIRST, then revoke old token
	tokens, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return nil, err
	}

	if err := s.sessions.RevokeRefreshToken(ctx, tokenHash); err != nil {
		slog.ErrorContext(ctx, "Failed to revoke old refresh token in redis", "error", err)
	}
	if err := s.repo.RevokeRefreshToken(ctx, tokenHash); err != nil {
		slog.ErrorContext(ctx, "Failed to revoke old refresh token in db", "error", err)
	}

	return tokens, nil
}

func (s *authService) Logout(ctx context.Context, refreshToken, accessToken string, claims *auth.Claims) error {
	var errs []error

	if refreshToken != "" {
		tokenHash := auth.HashToken(refreshToken)
		if err := s.sessions.RevokeRefreshToken(ctx, tokenHash); err != nil {
			slog.ErrorContext(ctx, "Failed to revoke refresh token", "error", err)
			errs = append(errs, err)
		}
		if err := s.repo.RevokeRefreshToken(ctx, tokenHash); err != nil {
			slog.ErrorContext(ctx, "Failed to revoke refresh token in db", "error", err)
			errs = append(errs, err)
		}
	}

	if claims != nil && accessToken != "" {
		tokenHash := auth.HashToken(accessToken)
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			if err := s.sessions.StoreBlacklistedToken(ctx, tokenHash, ttl); err != nil {
				slog.ErrorContext(ctx, "Failed to blacklist access token", "error", err)
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return apperr.WrapInternal("logout", errors.Join(errs...))
	}
	return nil
}

func (s *authService) issueTokenPair(ctx context.Context, user *models.User) (*auth.TokenPair, error) {
	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	tokens, err := auth.NewTokenPair(
		s.cfg.Secret,
		s.cfg.Issuer,
		user.ID,
		roleNames,
		s.cfg.TTLAccess,
		s.cfg.TTLRefresh,
	)
	if err != nil {
		return nil, apperr.WrapInternal("create token pair", err)
	}

	tokenHash := auth.HashToken(tokens.RefreshToken)

	if err := s.sessions.StoreRefreshToken(ctx, user.ID.String(), tokenHash, s.cfg.TTLRefresh); err != nil {
		return nil, apperr.WrapInternal("store refresh token", err)
	}

	if err := s.repo.CreateRefreshToken(ctx, &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(s.cfg.TTLRefresh),
	}); err != nil {
		slog.ErrorContext(ctx, "Failed to persist refresh token to DB", "error", err)
	}

	return tokens, nil
}

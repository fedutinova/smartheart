package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/mail"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/repository"
)

const (
	resetTokenBytes = 32
	resetTokenTTL   = 15 * time.Minute
)

type PasswordService interface {
	RequestReset(ctx context.Context, email string) error
	ConfirmReset(ctx context.Context, token, newPassword string) error
	ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error
}

type passwordService struct {
	repo        repository.Store
	sessions    auth.SessionService
	mailer      *mail.Sender
	frontendURL string
}

func NewPasswordService(repo repository.Store, sessions auth.SessionService, mailer *mail.Sender, cfg config.Config) PasswordService {
	return &passwordService{
		repo:        repo,
		sessions:    sessions,
		mailer:      mailer,
		frontendURL: cfg.FrontendURL,
	}
}

// RequestReset always returns nil to prevent email enumeration.
func (s *passwordService) RequestReset(ctx context.Context, email string) error {
	if email == "" {
		return fmt.Errorf("email is required: %w", apperr.ErrValidation)
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		// Don't reveal whether the email exists — return nil intentionally.
		slog.InfoContext(ctx, "Password reset requested for unknown email")
		return nil //nolint:nilerr // intentional: prevent email enumeration
	}

	rawToken, err := generateToken()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to generate reset token", "error", err)
		return nil
	}

	tokenHash := auth.HashToken(rawToken)

	if err := s.repo.InvalidateUserPasswordResetTokens(ctx, user.ID); err != nil {
		slog.ErrorContext(ctx, "Failed to invalidate old reset tokens", "user_id", user.ID, "error", err)
	}

	token := &models.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(resetTokenTTL),
	}
	if err := s.repo.CreatePasswordResetToken(ctx, token); err != nil {
		slog.ErrorContext(ctx, "Failed to create reset token", "user_id", user.ID, "error", err)
		return nil
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.frontendURL, rawToken)
	body := mail.PasswordResetEmail(resetLink)

	if err := s.mailer.Send(email, "SmartHeart — Сброс пароля", body); err != nil {
		slog.ErrorContext(ctx, "Failed to send password reset email", "email", email, "error", err)
		return nil
	}

	return nil
}

func (s *passwordService) ConfirmReset(ctx context.Context, rawToken, newPassword string) error {
	if rawToken == "" || newPassword == "" {
		return fmt.Errorf("token and new password are required: %w", apperr.ErrValidation)
	}

	if err := validatePassword(newPassword); err != nil {
		return err
	}

	tokenHash := auth.HashToken(rawToken)

	resetToken, err := s.repo.GetValidPasswordResetToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, apperr.ErrInvalidToken) {
			return fmt.Errorf("invalid or expired reset token: %w", apperr.ErrInvalidToken)
		}
		return apperr.WrapInternal("get reset token", err)
	}

	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return apperr.WrapInternal("hash password", err)
	}

	if err := s.repo.RunTx(ctx, func(tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		if err := txRepo.UpdateUserPassword(ctx, resetToken.UserID, passwordHash); err != nil {
			return err
		}
		return txRepo.MarkPasswordResetTokenUsed(ctx, resetToken.ID)
	}); err != nil {
		return apperr.WrapInternal("confirm password reset", err)
	}

	if err := s.sessions.RevokeAllUserTokens(ctx, resetToken.UserID.String()); err != nil {
		slog.ErrorContext(ctx, "Failed to revoke user sessions after password reset", "user_id", resetToken.UserID, "error", err)
	}
	if err := s.repo.RevokeAllUserRefreshTokens(ctx, resetToken.UserID); err != nil {
		slog.ErrorContext(ctx, "Failed to revoke refresh tokens in DB after password reset", "user_id", resetToken.UserID, "error", err)
	}

	return nil
}

func (s *passwordService) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	if oldPassword == "" || newPassword == "" {
		return fmt.Errorf("old and new passwords are required: %w", apperr.ErrValidation)
	}

	if err := validatePassword(newPassword); err != nil {
		return err
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return apperr.WrapInternal("get user", err)
	}

	if !auth.CheckPassword(oldPassword, user.PasswordHash) {
		return fmt.Errorf("old password is incorrect: %w", apperr.ErrInvalidCredentials)
	}

	passwordHash, err := auth.HashPassword(newPassword)
	if err != nil {
		return apperr.WrapInternal("hash password", err)
	}

	if err := s.repo.UpdateUserPassword(ctx, userID, passwordHash); err != nil {
		return apperr.WrapInternal("update password", err)
	}

	uidStr := userID.String()
	if err := s.sessions.RevokeAllUserTokens(ctx, uidStr); err != nil {
		slog.ErrorContext(ctx, "Failed to revoke user sessions after password change", "user_id", userID, "error", err)
	}
	if err := s.repo.RevokeAllUserRefreshTokens(ctx, userID); err != nil {
		slog.ErrorContext(ctx, "Failed to revoke refresh tokens in DB after password change", "user_id", userID, "error", err)
	}

	return nil
}

func generateToken() (string, error) {
	b := make([]byte, resetTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("password must be at least %d characters: %w", minPasswordLen, apperr.ErrValidation)
	}
	if len(password) > maxPasswordLen {
		return fmt.Errorf("password must not exceed %d bytes: %w", maxPasswordLen, apperr.ErrValidation)
	}
	if !passwordASCIIOnly.MatchString(password) {
		return fmt.Errorf("password must contain only English letters, digits, and symbols (no spaces): %w", apperr.ErrValidation)
	}
	return nil
}

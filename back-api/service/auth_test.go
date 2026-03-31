package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	authmocks "github.com/fedutinova/smartheart/back-api/auth/mocks"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/models"
	repomocks "github.com/fedutinova/smartheart/back-api/repository/mocks"
)

func jwt5ExpiresAt(t time.Time) *jwt.NumericDate {
	return jwt.NewNumericDate(t)
}

func newAuthService(t *testing.T) (*authService, *repomocks.MockStore, *authmocks.MockSessionService) {
	repo := repomocks.NewMockStore(t)
	sessions := authmocks.NewMockSessionService(t)
	cfg := config.JWTConfig{
		Secret:     "test-secret-that-is-long-enough-for-hs256",
		Issuer:     "test",
		TTLAccess:  15 * time.Minute,
		TTLRefresh: 24 * time.Hour,
	}
	svc := NewAuthService(repo, sessions, cfg).(*authService)
	return svc, repo, sessions
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	svc, repo, _ := newAuthService(t)
	ctx := context.Background()

	// WithTx returns the same mock so that CreateUser/AssignRoleToUser
	// expectations on `repo` are matched inside the transaction callback.
	repo.EXPECT().
		WithTx(mock.Anything).
		Return(repo)

	repo.EXPECT().
		RunTx(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, fn func(pgx.Tx) error) error {
			return fn(nil)
		})

	repo.EXPECT().
		CreateUser(mock.Anything, mock.Anything).
		Run(func(_ context.Context, user *models.User) {
			assert.Equal(t, "testuser", user.Username)
			assert.Equal(t, "test@example.com", user.Email)
			assert.NotEmpty(t, user.PasswordHash)
			user.ID = uuid.New()
		}).
		Return(nil)

	repo.EXPECT().
		AssignRoleToUser(mock.Anything, mock.Anything, auth.RoleUser).
		Return(nil)

	id, err := svc.Register(ctx, "testuser", "test@example.com", "strongpassword123")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, id)
}

func TestRegister_EmptyFields(t *testing.T) {
	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		username string
		email    string
		password string
	}{
		{"empty username", "", "test@example.com", "strongpassword123"},
		{"empty email", "testuser", "", "strongpassword123"},
		{"empty password", "testuser", "test@example.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Register(ctx, tt.username, tt.email, tt.password)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrValidation)
		})
	}
}

func TestRegister_UsernameTooLong(t *testing.T) {
	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	longName := make([]byte, maxUsernameLen+1)
	for i := range longName {
		longName[i] = 'a'
	}

	_, err := svc.Register(ctx, string(longName), "test@example.com", "strongpassword123")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestRegister_InvalidEmail(t *testing.T) {
	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	_, err := svc.Register(ctx, "testuser", "not-an-email", "strongpassword123")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestRegister_PasswordTooShort(t *testing.T) {
	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	_, err := svc.Register(ctx, "testuser", "test@example.com", "short")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestRegister_PasswordTooLong(t *testing.T) {
	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	longPassword := make([]byte, maxPasswordLen+1)
	for i := range longPassword {
		longPassword[i] = 'a'
	}

	_, err := svc.Register(ctx, "testuser", "test@example.com", string(longPassword))
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestRegister_TxFails(t *testing.T) {
	svc, repo, _ := newAuthService(t)
	ctx := context.Background()

	repo.EXPECT().
		RunTx(mock.Anything, mock.Anything).
		Return(errors.New("tx failed"))

	_, err := svc.Register(ctx, "testuser", "test@example.com", "strongpassword123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tx failed")
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	svc, repo, sessions := newAuthService(t)
	ctx := context.Background()

	password := "strongpassword123"
	hash, _ := auth.HashPassword(password)
	userID := uuid.New()

	sessions.EXPECT().
		IncrLoginAttempts(mock.Anything, "test@example.com", loginLockoutWindow).
		Return(int64(1), nil)

	repo.EXPECT().
		GetUserByEmail(mock.Anything, "test@example.com").
		Return(&models.User{
			ID:           userID,
			Email:        "test@example.com",
			PasswordHash: hash,
			Roles:        []models.Role{{Name: auth.RoleUser}},
		}, nil)

	sessions.EXPECT().
		ResetLoginAttempts(mock.Anything, "test@example.com").
		Return(nil)

	sessions.EXPECT().
		StoreRefreshToken(mock.Anything, userID.String(), mock.Anything, svc.cfg.TTLRefresh).
		Return(nil)

	repo.EXPECT().
		CreateRefreshToken(mock.Anything, mock.Anything).
		Return(nil)

	tokens, err := svc.Login(ctx, "test@example.com", password)
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
}

func TestLogin_EmptyFields(t *testing.T) {
	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{"empty email", "", "password123"},
		{"empty password", "test@example.com", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Login(ctx, tt.email, tt.password)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrValidation)
		})
	}
}

func TestLogin_TooManyAttempts(t *testing.T) {
	svc, _, sessions := newAuthService(t)
	ctx := context.Background()

	sessions.EXPECT().
		IncrLoginAttempts(mock.Anything, "test@example.com", loginLockoutWindow).
		Return(maxLoginAttempts+1, nil)

	_, err := svc.Login(ctx, "test@example.com", "password123")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTooManyAttempts)
}

func TestLogin_UserNotFound(t *testing.T) {
	svc, repo, sessions := newAuthService(t)
	ctx := context.Background()

	sessions.EXPECT().
		IncrLoginAttempts(mock.Anything, "test@example.com", loginLockoutWindow).
		Return(int64(1), nil)

	repo.EXPECT().
		GetUserByEmail(mock.Anything, "test@example.com").
		Return(nil, apperr.ErrNotFound)

	_, err := svc.Login(ctx, "test@example.com", "password123")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInvalidCredentials)
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, repo, sessions := newAuthService(t)
	ctx := context.Background()

	hash, _ := auth.HashPassword("correctpassword")

	sessions.EXPECT().
		IncrLoginAttempts(mock.Anything, "test@example.com", loginLockoutWindow).
		Return(int64(1), nil)

	repo.EXPECT().
		GetUserByEmail(mock.Anything, "test@example.com").
		Return(&models.User{
			ID:           uuid.New(),
			Email:        "test@example.com",
			PasswordHash: hash,
		}, nil)

	_, err := svc.Login(ctx, "test@example.com", "wrongpassword")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInvalidCredentials)
}

func TestLogin_RedisFailAllowsLogin(t *testing.T) {
	svc, repo, sessions := newAuthService(t)
	ctx := context.Background()

	password := "strongpassword123"
	hash, _ := auth.HashPassword(password)
	userID := uuid.New()

	// Redis failure should not block login
	sessions.EXPECT().
		IncrLoginAttempts(mock.Anything, "test@example.com", loginLockoutWindow).
		Return(int64(0), errors.New("redis down"))

	repo.EXPECT().
		GetUserByEmail(mock.Anything, "test@example.com").
		Return(&models.User{
			ID:           userID,
			Email:        "test@example.com",
			PasswordHash: hash,
			Roles:        []models.Role{{Name: auth.RoleUser}},
		}, nil)

	sessions.EXPECT().
		ResetLoginAttempts(mock.Anything, "test@example.com").
		Return(nil)

	sessions.EXPECT().
		StoreRefreshToken(mock.Anything, userID.String(), mock.Anything, svc.cfg.TTLRefresh).
		Return(nil)

	repo.EXPECT().
		CreateRefreshToken(mock.Anything, mock.Anything).
		Return(nil)

	tokens, err := svc.Login(ctx, "test@example.com", password)
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
}

// --- Refresh ---

func TestRefresh_Success(t *testing.T) {
	svc, repo, sessions := newAuthService(t)
	ctx := context.Background()

	userID := uuid.New()
	refreshToken := "valid-refresh-token"
	tokenHash := auth.HashToken(refreshToken)

	sessions.EXPECT().
		IncrLoginAttempts(mock.Anything, "refresh:"+tokenHash, refreshWindow).
		Return(int64(1), nil)

	sessions.EXPECT().
		GetRefreshTokenUserID(mock.Anything, tokenHash).
		Return(userID.String(), nil)

	repo.EXPECT().
		GetUserByID(mock.Anything, userID).
		Return(&models.User{
			ID:    userID,
			Roles: []models.Role{{Name: auth.RoleUser}},
		}, nil)

	sessions.EXPECT().
		StoreRefreshToken(mock.Anything, userID.String(), mock.Anything, svc.cfg.TTLRefresh).
		Return(nil)

	repo.EXPECT().
		CreateRefreshToken(mock.Anything, mock.Anything).
		Return(nil)

	sessions.EXPECT().
		RevokeRefreshToken(mock.Anything, tokenHash).
		Return(nil)

	tokens, err := svc.Refresh(ctx, refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
}

func TestRefresh_EmptyToken(t *testing.T) {
	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	_, err := svc.Refresh(ctx, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestRefresh_TooManyAttempts(t *testing.T) {
	svc, _, sessions := newAuthService(t)
	ctx := context.Background()

	refreshToken := "some-token"
	tokenHash := auth.HashToken(refreshToken)

	sessions.EXPECT().
		IncrLoginAttempts(mock.Anything, "refresh:"+tokenHash, refreshWindow).
		Return(maxRefreshAttempts+1, nil)

	_, err := svc.Refresh(ctx, refreshToken)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrTooManyAttempts)
}

func TestRefresh_InvalidToken(t *testing.T) {
	svc, _, sessions := newAuthService(t)
	ctx := context.Background()

	refreshToken := "invalid-token"
	tokenHash := auth.HashToken(refreshToken)

	sessions.EXPECT().
		IncrLoginAttempts(mock.Anything, "refresh:"+tokenHash, refreshWindow).
		Return(int64(1), nil)

	sessions.EXPECT().
		GetRefreshTokenUserID(mock.Anything, tokenHash).
		Return("", errors.New("not found"))

	_, err := svc.Refresh(ctx, refreshToken)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInvalidToken)
}

func TestRefresh_UserNotFound(t *testing.T) {
	svc, repo, sessions := newAuthService(t)
	ctx := context.Background()

	userID := uuid.New()
	refreshToken := "valid-token"
	tokenHash := auth.HashToken(refreshToken)

	sessions.EXPECT().
		IncrLoginAttempts(mock.Anything, "refresh:"+tokenHash, refreshWindow).
		Return(int64(1), nil)

	sessions.EXPECT().
		GetRefreshTokenUserID(mock.Anything, tokenHash).
		Return(userID.String(), nil)

	repo.EXPECT().
		GetUserByID(mock.Anything, userID).
		Return(nil, errors.New("not found"))

	_, err := svc.Refresh(ctx, refreshToken)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInvalidToken)
}

// --- Logout ---

func TestLogout_WithRefreshAndAccess(t *testing.T) {
	svc, repo, sessions := newAuthService(t)
	ctx := context.Background()

	refreshToken := "refresh-token"
	accessToken := "access-token"
	refreshHash := auth.HashToken(refreshToken)
	accessHash := auth.HashToken(accessToken)

	claims := &auth.Claims{
		UserID: uuid.New().String(),
	}
	claims.ExpiresAt = jwt5ExpiresAt(time.Now().Add(10 * time.Minute))

	sessions.EXPECT().
		RevokeRefreshToken(mock.Anything, refreshHash).
		Return(nil)

	repo.EXPECT().
		RevokeRefreshToken(mock.Anything, refreshHash).
		Return(nil)

	sessions.EXPECT().
		StoreBlacklistedToken(mock.Anything, accessHash, mock.Anything).
		Return(nil)

	err := svc.Logout(ctx, refreshToken, accessToken, claims)
	require.NoError(t, err)
}

func TestLogout_EmptyTokens(t *testing.T) {
	svc, _, _ := newAuthService(t)
	ctx := context.Background()

	err := svc.Logout(ctx, "", "", nil)
	require.NoError(t, err)
}

func TestLogout_OnlyRefreshToken(t *testing.T) {
	svc, repo, sessions := newAuthService(t)
	ctx := context.Background()

	refreshToken := "refresh-token"
	refreshHash := auth.HashToken(refreshToken)

	sessions.EXPECT().
		RevokeRefreshToken(mock.Anything, refreshHash).
		Return(nil)

	repo.EXPECT().
		RevokeRefreshToken(mock.Anything, refreshHash).
		Return(nil)

	err := svc.Logout(ctx, refreshToken, "", nil)
	require.NoError(t, err)
}

func TestLogout_ExpiredAccessToken(t *testing.T) {
	svc, repo, sessions := newAuthService(t)
	ctx := context.Background()

	refreshToken := "refresh-token"
	accessToken := "access-token"
	refreshHash := auth.HashToken(refreshToken)

	claims := &auth.Claims{
		UserID: uuid.New().String(),
	}
	// Already expired — TTL <= 0, so blacklisting should be skipped
	claims.ExpiresAt = jwt5ExpiresAt(time.Now().Add(-1 * time.Minute))

	sessions.EXPECT().
		RevokeRefreshToken(mock.Anything, refreshHash).
		Return(nil)

	repo.EXPECT().
		RevokeRefreshToken(mock.Anything, refreshHash).
		Return(nil)

	// StoreBlacklistedToken should NOT be called for expired token

	err := svc.Logout(ctx, refreshToken, accessToken, claims)
	require.NoError(t, err)
}

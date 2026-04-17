package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/auth"
	authmocks "github.com/fedutinova/smartheart/back-api/auth/mocks"
	"github.com/fedutinova/smartheart/back-api/config"
	"github.com/fedutinova/smartheart/back-api/mail"
	"github.com/fedutinova/smartheart/back-api/models"
	repomocks "github.com/fedutinova/smartheart/back-api/repository/mocks"
)

func newPasswordService(t *testing.T) (*passwordService, *repomocks.MockStore, *authmocks.MockSessionService) {
	repo := repomocks.NewMockStore(t)
	sessions := authmocks.NewMockSessionService(t)
	mailer := mail.NewSender(config.SMTPConfig{})
	cfg := config.Config{FrontendURL: "http://localhost:3000"}
	svc := NewPasswordService(repo, sessions, mailer, cfg).(*passwordService)
	return svc, repo, sessions
}

// ---------------------------------------------------------------------------
// RequestReset
// ---------------------------------------------------------------------------

func TestRequestReset_EmptyEmail(t *testing.T) {
	svc, _, _ := newPasswordService(t)

	err := svc.RequestReset(context.Background(), "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestRequestReset_UnknownEmail_ReturnsNil(t *testing.T) {
	svc, repo, _ := newPasswordService(t)

	repo.EXPECT().
		GetUserByEmail(mock.Anything, "unknown@example.com").
		Return(nil, errors.New("not found"))

	err := svc.RequestReset(context.Background(), "unknown@example.com")
	require.NoError(t, err, "must return nil to prevent email enumeration")
}

// ---------------------------------------------------------------------------
// ConfirmReset
// ---------------------------------------------------------------------------

func TestConfirmReset_EmptyToken(t *testing.T) {
	svc, _, _ := newPasswordService(t)

	err := svc.ConfirmReset(context.Background(), "", "newpassword123")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestConfirmReset_EmptyPassword(t *testing.T) {
	svc, _, _ := newPasswordService(t)

	err := svc.ConfirmReset(context.Background(), "sometoken", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestConfirmReset_WeakPassword(t *testing.T) {
	svc, _, _ := newPasswordService(t)

	err := svc.ConfirmReset(context.Background(), "sometoken", "short")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestConfirmReset_InvalidToken(t *testing.T) {
	svc, repo, _ := newPasswordService(t)

	repo.EXPECT().
		GetValidPasswordResetToken(mock.Anything, mock.Anything).
		Return(nil, apperr.ErrInvalidToken)

	err := svc.ConfirmReset(context.Background(), "badtoken0000000000000000000000000000000000000000000000000000000000", "strongpassword123")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInvalidToken)
}

func TestConfirmReset_DBError_Returns500(t *testing.T) {
	svc, repo, _ := newPasswordService(t)

	repo.EXPECT().
		GetValidPasswordResetToken(mock.Anything, mock.Anything).
		Return(nil, errors.New("connection refused"))

	err := svc.ConfirmReset(context.Background(), "badtoken0000000000000000000000000000000000000000000000000000000000", "strongpassword123")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInternal)
	assert.NotErrorIs(t, err, apperr.ErrInvalidToken, "DB errors must not be mapped to ErrInvalidToken")
}

func TestConfirmReset_Success(t *testing.T) {
	svc, repo, sessions := newPasswordService(t)
	ctx := context.Background()
	userID := uuid.New()
	tokenID := uuid.New()

	repo.EXPECT().
		GetValidPasswordResetToken(mock.Anything, mock.Anything).
		Return(&models.PasswordResetToken{ID: tokenID, UserID: userID}, nil)

	repo.EXPECT().WithTx(mock.Anything).Return(repo)
	repo.EXPECT().
		RunTx(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, fn func(pgx.Tx) error) error {
			return fn(nil)
		})

	repo.EXPECT().
		UpdateUserPassword(mock.Anything, userID, mock.AnythingOfType("string")).
		Return(nil)

	repo.EXPECT().
		MarkPasswordResetTokenUsed(mock.Anything, tokenID).
		Return(nil)

	sessions.EXPECT().
		RevokeAllUserTokens(mock.Anything, userID.String()).
		Return(nil)

	repo.EXPECT().
		RevokeAllUserRefreshTokens(mock.Anything, userID).
		Return(nil)

	err := svc.ConfirmReset(ctx, "validtoken000000000000000000000000000000000000000000000000000000", "strongpassword123")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// ChangePassword
// ---------------------------------------------------------------------------

func TestChangePassword_EmptyFields(t *testing.T) {
	svc, _, _ := newPasswordService(t)

	tests := []struct {
		name string
		old  string
		new  string
	}{
		{"empty old password", "", "strongpassword123"},
		{"empty new password", "oldpassword123", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.ChangePassword(context.Background(), uuid.New(), tt.old, tt.new)
			require.Error(t, err)
			assert.ErrorIs(t, err, apperr.ErrValidation)
		})
	}
}

func TestChangePassword_WeakNewPassword(t *testing.T) {
	svc, _, _ := newPasswordService(t)

	err := svc.ChangePassword(context.Background(), uuid.New(), "oldpassword123", "short")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrValidation)
}

func TestChangePassword_WrongOldPassword(t *testing.T) {
	svc, repo, _ := newPasswordService(t)
	userID := uuid.New()
	hash, _ := auth.HashPassword("correctpassword1")

	repo.EXPECT().
		GetUserByID(mock.Anything, userID).
		Return(&models.User{ID: userID, PasswordHash: hash}, nil)

	err := svc.ChangePassword(context.Background(), userID, "wrongpassword1", "newstrongpassword1")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrInvalidCredentials)
}

func TestChangePassword_Success(t *testing.T) {
	svc, repo, sessions := newPasswordService(t)
	ctx := context.Background()
	userID := uuid.New()
	oldPassword := "oldpassword123"
	hash, _ := auth.HashPassword(oldPassword)

	repo.EXPECT().
		GetUserByID(mock.Anything, userID).
		Return(&models.User{ID: userID, PasswordHash: hash}, nil)

	repo.EXPECT().
		UpdateUserPassword(mock.Anything, userID, mock.AnythingOfType("string")).
		Return(nil)

	sessions.EXPECT().
		RevokeAllUserTokens(mock.Anything, userID.String()).
		Return(nil)

	repo.EXPECT().
		RevokeAllUserRefreshTokens(mock.Anything, userID).
		Return(nil)

	err := svc.ChangePassword(ctx, userID, oldPassword, "newstrongpassword1")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// validatePassword
// ---------------------------------------------------------------------------

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		pass    string
		wantErr bool
	}{
		{"valid", "strongpass123", false},
		{"too short", "short", true},
		{"too long", string(make([]byte, maxPasswordLen+1)), true},
		{"has spaces", "strong pass 123", true},
		{"has unicode", "пароль12345", true},
		{"min length exactly", "abcdefghij", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.pass)
			if tt.wantErr {
				assert.ErrorIs(t, err, apperr.ErrValidation)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

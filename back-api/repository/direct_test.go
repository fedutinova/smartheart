package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/models"
)

func TestCreateRefreshToken_AssignsIDWhenMissing(t *testing.T) {
	repo := NewTxScoped(stubQuerier{})
	token := &models.RefreshToken{
		UserID:    uuid.New(),
		TokenHash: "hashed-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	err := repo.CreateRefreshToken(context.Background(), token)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, token.ID)
}

func TestGetRefreshToken_ReturnsInvalidTokenOnNoRows(t *testing.T) {
	repo := NewTxScoped(stubQuerier{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return stubRow{
				scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	})

	token, err := repo.GetRefreshToken(context.Background(), "missing")
	require.Error(t, err)
	assert.Nil(t, token)
	assert.ErrorIs(t, err, apperr.ErrInvalidToken)
}

func TestGetRefreshToken_WrapsUnexpectedQueryRowError(t *testing.T) {
	repo := NewTxScoped(stubQuerier{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return stubRow{
				scanFn: func(dest ...any) error {
					return errors.New("db unavailable")
				},
			}
		},
	})

	token, err := repo.GetRefreshToken(context.Background(), "broken")
	require.Error(t, err)
	assert.Nil(t, token)
	require.ErrorContains(t, err, "failed to get refresh token")
	assert.NotErrorIs(t, err, apperr.ErrInvalidToken)
}

func TestGetDailyUsage_ReturnsZeroOnNoRows(t *testing.T) {
	repo := NewTxScoped(stubQuerier{
		queryRowFn: func(context.Context, string, ...any) pgx.Row {
			return stubRow{
				scanFn: func(dest ...any) error {
					return pgx.ErrNoRows
				},
			}
		},
	})

	count, err := repo.GetDailyUsage(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestUpdateRequestStatus_RejectsUnknownStatus(t *testing.T) {
	repo := NewTxScoped(stubQuerier{})

	err := repo.UpdateRequestStatus(context.Background(), uuid.New(), "queued")
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid request status")
}

func TestUpdateRequestStatus_ReturnsRequestNotFoundWhenNothingUpdated(t *testing.T) {
	repo := NewTxScoped(stubQuerier{
		execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		},
	})

	err := repo.UpdateRequestStatus(context.Background(), uuid.New(), models.StatusCompleted)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrRequestNotFound)
}

func TestCreatePayment_MapsUniqueViolationToConflict(t *testing.T) {
	repo := NewTxScoped(stubQuerier{
		execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, &pgconn.PgError{Code: "23505"}
		},
	})
	payment := &models.Payment{
		ID:            uuid.New(),
		UserID:        uuid.New(),
		YooKassaID:    "pay_123",
		Status:        models.PaymentPending,
		AmountKopecks: 9900,
		Description:   "subscription",
		AnalysesCount: 0,
		PaymentType:   models.PaymentTypeSubscription,
	}

	err := repo.CreatePayment(context.Background(), payment)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrConflict)
}

func TestCancelStalePayments_ReturnsAffectedRows(t *testing.T) {
	repo := NewTxScoped(stubQuerier{
		execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("UPDATE 3"), nil
		},
	})

	count, err := repo.CancelStalePayments(context.Background(), 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

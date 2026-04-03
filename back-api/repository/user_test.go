package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fedutinova/smartheart/back-api/apperr"
	"github.com/fedutinova/smartheart/back-api/models"
)

func TestCreateUser_AssignsIDWhenMissing(t *testing.T) {
	repo := NewTxScoped(stubQuerier{})
	user := &models.User{
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
	}

	err := repo.CreateUser(context.Background(), user)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID)
}

func TestCreateUser_MapsUniqueViolationToConflict(t *testing.T) {
	repo := NewTxScoped(stubQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, &pgconn.PgError{Code: "23505"}
		},
	})
	user := &models.User{
		ID:           uuid.New(),
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
	}

	err := repo.CreateUser(context.Background(), user)
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.ErrConflict)
}

func TestCreateUser_WrapsUnexpectedExecError(t *testing.T) {
	repo := NewTxScoped(stubQuerier{
		execFn: func(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("db unavailable")
		},
	})
	user := &models.User{
		ID:           uuid.New(),
		Username:     "alice",
		Email:        "alice@example.com",
		PasswordHash: "hash",
	}

	err := repo.CreateUser(context.Background(), user)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create user")
	assert.NotErrorIs(t, err, apperr.ErrConflict)
}

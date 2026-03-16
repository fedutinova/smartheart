package repository

import (
	"context"
	"time"

	"github.com/fedutinova/smartheart/internal/database"
	"github.com/fedutinova/smartheart/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UserRepo provides user-related data access.
type UserRepo interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error)
	AssignRoleToUser(ctx context.Context, userID uuid.UUID, roleName string) error
}

// RequestRepo provides request/file/response data access.
type RequestRepo interface {
	CreateRequest(ctx context.Context, req *models.Request) error
	GetRequestByID(ctx context.Context, id uuid.UUID) (*models.Request, error)
	GetRequestsByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Request, error)
	UpdateRequestStatus(ctx context.Context, requestID uuid.UUID, status string) error
	CreateFile(ctx context.Context, file *models.File) error
	GetFilesByRequestID(ctx context.Context, requestID uuid.UUID) ([]models.File, error)
	CreateResponse(ctx context.Context, resp *models.Response) error
	GetResponseByRequestID(ctx context.Context, requestID uuid.UUID) (*models.Response, error)
}

// TokenRepo provides refresh-token data access.
type TokenRepo interface {
	CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
}

// RoleRepo provides role/permission data access.
type RoleRepo interface {
	LoadRolePermissions(ctx context.Context) (map[string][]string, error)
}

// Store is the composite interface that embeds all focused interfaces.
type Store interface {
	UserRepo
	RequestRepo
	TokenRepo
	RoleRepo

	// Transaction support
	WithTx(tx pgx.Tx) Store
	DB() *database.DB
}

// Repository provides data access methods backed by PostgreSQL.
type Repository struct {
	db      *database.DB
	querier database.Querier // can be pool or transaction
}

// New creates a new Repository.
func New(db *database.DB, opts ...func(*Repository)) *Repository {
	r := &Repository{
		db:      db,
		querier: db.Pool(),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// WithQueryTimeout wraps the default querier with a context timeout.
func WithQueryTimeout(d time.Duration) func(*Repository) {
	return func(r *Repository) {
		r.querier = database.NewTimeoutQuerier(r.querier, d)
	}
}

// NewWithQuerier creates a Repository backed by a specific Querier (e.g. a transaction).
// This allows callers that only need a focused interface (like RequestRepo) to create
// a transaction-scoped repository without depending on the full Store interface.
func NewWithQuerier(db *database.DB, q database.Querier) *Repository {
	return &Repository{
		db:      db,
		querier: q,
	}
}

// WithTx creates a new Repository that uses the given transaction.
func (r *Repository) WithTx(tx pgx.Tx) Store {
	return &Repository{
		db:      r.db,
		querier: tx,
	}
}

// DB returns the underlying database connection.
func (r *Repository) DB() *database.DB {
	return r.db
}

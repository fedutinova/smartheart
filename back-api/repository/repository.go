package repository

import (
	"context"
	"time"

	"github.com/fedutinova/smartheart/back-api/database"
	"github.com/fedutinova/smartheart/back-api/models"
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
	CountRequestsByUserID(ctx context.Context, userID uuid.UUID) (int, error)
	GetRecentRequestsWithResponses(ctx context.Context, userID uuid.UUID, limit int) ([]models.Request, error)
	UpdateRequestStatus(ctx context.Context, requestID uuid.UUID, status string) error
	CreateFile(ctx context.Context, file *models.File) error
	GetFilesByRequestID(ctx context.Context, requestID uuid.UUID) ([]models.File, error)
	CreateResponse(ctx context.Context, resp *models.Response) error
	GetResponseByRequestID(ctx context.Context, requestID uuid.UUID) (*models.Response, error)
}

// QuotaRepo provides daily usage quota data access.
type QuotaRepo interface {
	IncrementDailyUsage(ctx context.Context, userID uuid.UUID) (int, error)
	GetDailyUsage(ctx context.Context, userID uuid.UUID) (int, error)
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

// RAGFeedbackRepo provides RAG feedback data access.
type RAGFeedbackRepo interface {
	CreateRAGFeedback(ctx context.Context, feedback *models.RAGFeedback) error
}

// Store is the composite interface that embeds all focused interfaces.
type Store interface {
	UserRepo
	RequestRepo
	TokenRepo
	RoleRepo
	QuotaRepo
	RAGFeedbackRepo

	// Transaction support
	RunTx(ctx context.Context, fn func(tx pgx.Tx) error) error
	WithTx(tx pgx.Tx) Store

	// Ping checks that the underlying database connection is alive.
	Ping(ctx context.Context) error
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
// The db parameter is optional — pass nil for tx-scoped repos that don't need DB() or WithTx().
func NewWithQuerier(db *database.DB, q database.Querier) *Repository {
	return &Repository{
		db:      db,
		querier: q,
	}
}

// NewTxScoped creates a transaction-scoped Repository.
// Unlike NewWithQuerier, it does not require a *DB reference, making it
// suitable for use with the TxBeginner interface where *DB is not available.
// The returned repo must not call DB() or WithTx().
func NewTxScoped(q database.Querier) *Repository {
	return &Repository{querier: q}
}

// WithTx creates a new Repository that uses the given transaction.
func (r *Repository) WithTx(tx pgx.Tx) Store {
	return &Repository{
		db:      r.db,
		querier: tx,
	}
}

// RunTx executes fn inside a database transaction.
func (r *Repository) RunTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return r.db.WithTx(ctx, fn)
}

// Ping checks that the underlying database connection is alive.
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Pool().Ping(ctx)
}

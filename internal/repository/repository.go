package repository

import (
	"github.com/fedutinova/smartheart/internal/database"
	"github.com/jackc/pgx/v5"
)

// Repository provides data access methods
type Repository struct {
	db *database.DB
	q  database.Querier // can be pool or transaction
}

// New creates a new Repository
func New(db *database.DB) *Repository {
	return &Repository{
		db: db,
		q:  db.Pool(),
	}
}

// WithTx creates a new Repository that uses the given transaction
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{
		db: r.db,
		q:  tx,
	}
}

// DB returns the underlying database connection
func (r *Repository) DB() *database.DB {
	return r.db
}

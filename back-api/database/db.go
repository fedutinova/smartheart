package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is an interface that can execute queries (pool or transaction)
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Tx is an alias for pgx.Tx for convenience
type Tx = pgx.Tx

// TxBeginner abstracts transaction support so callers don't depend on *DB directly.
type TxBeginner interface {
	WithTx(ctx context.Context, fn func(tx Tx) error) error
}

type DB struct {
	pool *pgxpool.Pool
}

// PoolConfig holds optional connection pool tuning parameters.
type PoolConfig struct {
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

func NewDB(ctx context.Context, databaseURL string, opts ...func(*PoolConfig)) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	pc := PoolConfig{}
	for _, o := range opts {
		o(&pc)
	}
	if pc.MaxConns > 0 {
		cfg.MaxConns = pc.MaxConns
	}
	if pc.MinConns > 0 {
		cfg.MinConns = pc.MinConns
	}
	if pc.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = pc.MaxConnLifetime
	}
	if pc.MaxConnIdleTime > 0 {
		cfg.MaxConnIdleTime = pc.MaxConnIdleTime
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	slog.Info("database connection established",
		"max_conns", cfg.MaxConns,
		"min_conns", cfg.MinConns)
	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// WithTx executes a function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, it's committed.
func (db *DB) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // re-throw panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			slog.Error("failed to rollback transaction", "error", rbErr)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// BeginTx starts a new transaction and returns it.
// The caller is responsible for calling Commit() or Rollback().
func (db *DB) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return db.pool.Begin(ctx)
}

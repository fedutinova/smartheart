package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TimeoutQuerier wraps a Querier and applies a default timeout to every
// query context that doesn't already have a deadline.
type TimeoutQuerier struct {
	inner   Querier
	timeout time.Duration
}

// NewTimeoutQuerier returns a Querier that enforces a default timeout.
// If timeout <= 0, the inner querier is returned unchanged.
func NewTimeoutQuerier(q Querier, timeout time.Duration) Querier {
	if timeout <= 0 {
		return q
	}
	return &TimeoutQuerier{inner: q, timeout: timeout}
}

func (t *TimeoutQuerier) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {} // caller already set a deadline
	}
	return context.WithTimeout(ctx, t.timeout)
}

func (t *TimeoutQuerier) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	ctx, cancel := t.withTimeout(ctx)
	defer cancel()
	return t.inner.Exec(ctx, sql, arguments...)
}

// Query wraps the inner Query with a timeout context.
// The cancel func is deferred to rows.Close() so the context stays alive
// while the caller iterates over results.
func (t *TimeoutQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	ctx, cancel := t.withTimeout(ctx)
	rows, err := t.inner.Query(ctx, sql, args...)
	if err != nil {
		cancel()
		return nil, err
	}
	return &cancelRows{Rows: rows, cancel: cancel}, nil
}

// cancelRows wraps pgx.Rows and calls cancel when the rows are closed.
type cancelRows struct {
	pgx.Rows
	cancel context.CancelFunc
}

func (r *cancelRows) Close() {
	r.Rows.Close()
	r.cancel()
}

// QueryRow wraps the inner QueryRow with a timeout context.
// The cancel func is deferred to Scan() so the context stays alive
// until the row data is actually read.
func (t *TimeoutQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	ctx, cancel := t.withTimeout(ctx)
	row := t.inner.QueryRow(ctx, sql, args...)
	return &cancelRow{row: row, cancel: cancel}
}

// cancelRow wraps pgx.Row and calls cancel after Scan completes.
type cancelRow struct {
	row    pgx.Row
	cancel context.CancelFunc
}

func (r *cancelRow) Scan(dest ...any) error {
	defer r.cancel()
	return r.row.Scan(dest...)
}

package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type stubQuerier struct {
	execFn     func(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	queryRowFn func(ctx context.Context, sql string, arguments ...any) pgx.Row
}

func (s stubQuerier) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if s.execFn != nil {
		return s.execFn(ctx, sql, arguments...)
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (s stubQuerier) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	if s.queryFn != nil {
		return s.queryFn(ctx, sql, arguments...)
	}
	panic("Query not implemented in stubQuerier")
}

func (s stubQuerier) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	if s.queryRowFn != nil {
		return s.queryRowFn(ctx, sql, arguments...)
	}
	panic("QueryRow not implemented in stubQuerier")
}

type stubRow struct {
	scanFn func(dest ...any) error
}

func (r stubRow) Scan(dest ...any) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return nil
}

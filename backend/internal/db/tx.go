package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type txKey struct{}

// WithTx returns a context whose InTx calls run inside tx, each as a
// savepoint, so several service calls commit or roll back together.
func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// InTx runs fn in a transaction: a new one on q, or a savepoint of the
// transaction ctx carries (see WithTx). It commits when fn returns nil and
// rolls back otherwise; a savepoint rolled back leaves the outer
// transaction as it was before fn.
func InTx(ctx context.Context, q Querier, fn func(pgx.Tx) error) error {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return pgx.BeginFunc(ctx, tx, fn)
	}
	return pgx.BeginFunc(ctx, q, fn)
}

// Package db carries the process's database handle on a context and runs
// transactions against it.
//
// This replaces core-go/database, whose query builders select columns from the
// GraphQL fields a resolver was asked for and therefore pull in gqlgen. Only
// the API server needed that; what is left of this service needs a handle and
// a transaction wrapper.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type contextKey struct {
	name string
}

var ctxKey = &contextKey{"db"}

func Context(ctx context.Context, db *sql.DB) context.Context {
	return context.WithValue(ctx, ctxKey, db)
}

// Panics if no handle was placed on the context: that is a programming error,
// not a runtime condition.
func ForContext(ctx context.Context) *sql.DB {
	db, ok := ctx.Value(ctxKey).(*sql.DB)
	if !ok {
		panic(errors.New("no database on this context"))
	}
	return db
}

// Run fn in a transaction, committing it if fn returns nil and rolling it back
// otherwise. Unlike core-go/database this reports a failed commit or rollback
// rather than panicking, so a database blip bounces one message instead of
// taking the ingress down with it.
func WithTx(ctx context.Context, opts *sql.TxOptions, fn func(tx *sql.Tx) error) error {
	tx, err := ForContext(ctx).BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		if e := tx.Rollback(); e != nil && !errors.Is(e, sql.ErrTxDone) {
			return fmt.Errorf("%w (rollback failed: %v)", err, e)
		}
		return err
	}

	return tx.Commit()
}

func WithReadOnlyTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return WithTx(ctx, &sql.TxOptions{ReadOnly: true}, fn)
}

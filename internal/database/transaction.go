package database

import "context"

type TxFunc func(ctx context.Context) error

func WithTx(ctx context.Context, begin func(context.Context) (Tx, error), fn TxFunc) error {
	tx, err := begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

type Tx interface {
	Commit(context.Context) error
	Rollback(context.Context) error
}

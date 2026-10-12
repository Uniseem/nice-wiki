package store

import (
	"context"
	"database/sql"
	"errors"
)

func Transaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if rollback := tx.Rollback(); rollback != nil && !errors.Is(rollback, sql.ErrTxDone) {
			err = errors.Join(err, rollback)
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

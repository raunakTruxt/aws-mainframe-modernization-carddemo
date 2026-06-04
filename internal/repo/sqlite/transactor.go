package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// Transactor wraps a TransactionRepository+AccountRepository write pair in a
// single SQLite transaction. It satisfies service/transaction.Transactor via
// Go's structural typing (no import of that package is needed).
type Transactor struct{ db *sql.DB }

// NewTransactor returns a Transactor backed by db.
func NewTransactor(db *sql.DB) *Transactor { return &Transactor{db: db} }

// Do begins a database transaction, passes tx-scoped TransactionRepository and
// AccountRepository to fn, and commits on success or rolls back on error.
// This is the Go equivalent of the CICS syncpoint that made COTRN02C and
// COBIL00C atomic: the insert and balance-update are a single unit of work.
func (t *Transactor) Do(
	ctx context.Context,
	fn func(txns repo.TransactionRepository, accts repo.AccountRepository) error,
) error {
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: transactor begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := fn(NewTransactionStore(tx), NewAccountStore(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

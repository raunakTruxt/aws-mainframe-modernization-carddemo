package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// scanner is satisfied by both *sql.Row and *sql.Rows, so a single scan helper
// serves point reads and row iteration.
type scanner interface {
	Scan(dest ...any) error
}

// requireAffected maps a zero-row UPDATE/DELETE to repo.ErrNotFound, matching
// the VSAM "record not found" condition on REWRITE/DELETE.
func requireAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: rows affected: %w", err)
	}
	if n == 0 {
		return repo.ErrNotFound
	}
	return nil
}

// isUniqueViolation reports whether err is a SQLite primary-key/unique conflict.
// modernc.org/sqlite surfaces these as text containing "UNIQUE constraint".
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint")
}

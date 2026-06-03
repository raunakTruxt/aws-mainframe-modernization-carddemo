package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// TranTypeStore is the SQLite-backed repo.TranTypeRepository.
type TranTypeStore struct{ db querier }

// NewTranTypeStore returns a TranTypeStore over db (accepts *sql.DB or *sql.Tx).
func NewTranTypeStore(db querier) *TranTypeStore { return &TranTypeStore{db: db} }

const tranTypeColumns = `tran_type, tran_type_desc`

func (s *TranTypeStore) Get(ctx context.Context, tranType string) (*domain.TranTypeRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+tranTypeColumns+` FROM tran_types WHERE tran_type = ?`, tranType)
	return scanTranType(row)
}

func (s *TranTypeStore) Create(ctx context.Context, r *domain.TranTypeRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO tran_types (`+tranTypeColumns+`) VALUES (?, ?)`, r.TranType, r.TranTypeDesc)
	if isUniqueViolation(err) {
		return repo.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("sqlite: trantype create: %w", err)
	}
	return nil
}

func (s *TranTypeStore) Update(ctx context.Context, r *domain.TranTypeRecord) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tran_types SET tran_type_desc = ? WHERE tran_type = ?`, r.TranTypeDesc, r.TranType)
	if err != nil {
		return fmt.Errorf("sqlite: trantype update: %w", err)
	}
	return requireAffected(res)
}

func (s *TranTypeStore) Delete(ctx context.Context, tranType string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tran_types WHERE tran_type = ?`, tranType)
	if err != nil {
		return fmt.Errorf("sqlite: trantype delete: %w", err)
	}
	return requireAffected(res)
}

func (s *TranTypeStore) List(ctx context.Context) ([]*domain.TranTypeRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+tranTypeColumns+` FROM tran_types ORDER BY tran_type`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: trantype list: %w", err)
	}
	defer rows.Close()

	var out []*domain.TranTypeRecord
	for rows.Next() {
		rec, err := scanTranType(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func scanTranType(sc scanner) (*domain.TranTypeRecord, error) {
	var r domain.TranTypeRecord
	err := sc.Scan(&r.TranType, &r.TranTypeDesc)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: trantype scan: %w", err)
	}
	return &r, nil
}

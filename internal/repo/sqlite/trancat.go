package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// TranCatStore is the SQLite-backed repo.TranCatRepository.
type TranCatStore struct{ db *sql.DB }

// NewTranCatStore returns a TranCatStore over db.
func NewTranCatStore(db *sql.DB) *TranCatStore { return &TranCatStore{db: db} }

const tranCatColumns = `tran_type_cd, tran_cat_cd, tran_cat_type_desc`

func (s *TranCatStore) Get(ctx context.Context, typeCD string, catCD int64) (*domain.TranCatRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+tranCatColumns+` FROM tran_cats WHERE tran_type_cd = ? AND tran_cat_cd = ?`, typeCD, catCD)
	return scanTranCat(row)
}

func (s *TranCatStore) GetByType(ctx context.Context, typeCD string) ([]*domain.TranCatRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+tranCatColumns+` FROM tran_cats WHERE tran_type_cd = ? ORDER BY tran_cat_cd`, typeCD)
	if err != nil {
		return nil, fmt.Errorf("sqlite: trancat get-by-type: %w", err)
	}
	defer rows.Close()
	return collectTranCats(rows)
}

func (s *TranCatStore) Create(ctx context.Context, r *domain.TranCatRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO tran_cats (`+tranCatColumns+`) VALUES (?, ?, ?)`,
		r.TranTypeCode, r.TranCatCode, r.TranCatTypeDesc)
	if isUniqueViolation(err) {
		return repo.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("sqlite: trancat create: %w", err)
	}
	return nil
}

func (s *TranCatStore) Update(ctx context.Context, r *domain.TranCatRecord) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tran_cats SET tran_cat_type_desc = ? WHERE tran_type_cd = ? AND tran_cat_cd = ?`,
		r.TranCatTypeDesc, r.TranTypeCode, r.TranCatCode)
	if err != nil {
		return fmt.Errorf("sqlite: trancat update: %w", err)
	}
	return requireAffected(res)
}

func (s *TranCatStore) Delete(ctx context.Context, typeCD string, catCD int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tran_cats WHERE tran_type_cd = ? AND tran_cat_cd = ?`, typeCD, catCD)
	if err != nil {
		return fmt.Errorf("sqlite: trancat delete: %w", err)
	}
	return requireAffected(res)
}

func (s *TranCatStore) List(ctx context.Context) ([]*domain.TranCatRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+tranCatColumns+` FROM tran_cats ORDER BY tran_type_cd, tran_cat_cd`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: trancat list: %w", err)
	}
	defer rows.Close()
	return collectTranCats(rows)
}

func collectTranCats(rows *sql.Rows) ([]*domain.TranCatRecord, error) {
	var out []*domain.TranCatRecord
	for rows.Next() {
		rec, err := scanTranCat(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func scanTranCat(sc scanner) (*domain.TranCatRecord, error) {
	var r domain.TranCatRecord
	err := sc.Scan(&r.TranTypeCode, &r.TranCatCode, &r.TranCatTypeDesc)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: trancat scan: %w", err)
	}
	return &r, nil
}

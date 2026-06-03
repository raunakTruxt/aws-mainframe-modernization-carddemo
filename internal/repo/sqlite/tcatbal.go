package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/shopspring/decimal"
)

// TranCatBalStore is the SQLite-backed repo.TranCatBalRepository.
type TranCatBalStore struct{ db *sql.DB }

// NewTranCatBalStore returns a TranCatBalStore over db.
func NewTranCatBalStore(db *sql.DB) *TranCatBalStore { return &TranCatBalStore{db: db} }

const tranCatBalColumns = `trancat_acct_id, trancat_type_cd, trancat_cd, tran_cat_balance`

func (s *TranCatBalStore) Get(ctx context.Context, acctID int64, typeCD string, catCD int64) (*domain.TranCatBalRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+tranCatBalColumns+` FROM tran_cat_bals WHERE trancat_acct_id = ? AND trancat_type_cd = ? AND trancat_cd = ?`, acctID, typeCD, catCD)
	return scanTranCatBal(row)
}

func (s *TranCatBalStore) GetByAccount(ctx context.Context, acctID int64) ([]*domain.TranCatBalRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+tranCatBalColumns+` FROM tran_cat_bals WHERE trancat_acct_id = ? ORDER BY trancat_type_cd, trancat_cd`, acctID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: tcatbal get-by-account: %w", err)
	}
	defer rows.Close()
	return collectTranCatBals(rows)
}

func (s *TranCatBalStore) Create(ctx context.Context, r *domain.TranCatBalRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO tran_cat_bals (`+tranCatBalColumns+`) VALUES (?, ?, ?, ?)`,
		r.TrancatAcctID, r.TrancatTypeCD, r.TrancatCode, r.TranCatBalance.String())
	if isUniqueViolation(err) {
		return repo.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("sqlite: tcatbal create: %w", err)
	}
	return nil
}

func (s *TranCatBalStore) Update(ctx context.Context, r *domain.TranCatBalRecord) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tran_cat_bals SET tran_cat_balance = ? WHERE trancat_acct_id = ? AND trancat_type_cd = ? AND trancat_cd = ?`,
		r.TranCatBalance.String(), r.TrancatAcctID, r.TrancatTypeCD, r.TrancatCode)
	if err != nil {
		return fmt.Errorf("sqlite: tcatbal update: %w", err)
	}
	return requireAffected(res)
}

func (s *TranCatBalStore) Upsert(ctx context.Context, r *domain.TranCatBalRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO tran_cat_bals (`+tranCatBalColumns+`) VALUES (?, ?, ?, ?) ON CONFLICT (trancat_acct_id, trancat_type_cd, trancat_cd) DO UPDATE SET tran_cat_balance = excluded.tran_cat_balance`,
		r.TrancatAcctID, r.TrancatTypeCD, r.TrancatCode, r.TranCatBalance.String())
	if err != nil {
		return fmt.Errorf("sqlite: tcatbal upsert: %w", err)
	}
	return nil
}

func (s *TranCatBalStore) Delete(ctx context.Context, acctID int64, typeCD string, catCD int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tran_cat_bals WHERE trancat_acct_id = ? AND trancat_type_cd = ? AND trancat_cd = ?`, acctID, typeCD, catCD)
	if err != nil {
		return fmt.Errorf("sqlite: tcatbal delete: %w", err)
	}
	return requireAffected(res)
}

func (s *TranCatBalStore) List(ctx context.Context) ([]*domain.TranCatBalRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+tranCatBalColumns+` FROM tran_cat_bals ORDER BY trancat_acct_id, trancat_type_cd, trancat_cd`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: tcatbal list: %w", err)
	}
	defer rows.Close()
	return collectTranCatBals(rows)
}

func collectTranCatBals(rows *sql.Rows) ([]*domain.TranCatBalRecord, error) {
	var out []*domain.TranCatBalRecord
	for rows.Next() {
		rec, err := scanTranCatBal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func scanTranCatBal(sc scanner) (*domain.TranCatBalRecord, error) {
	var (
		r   domain.TranCatBalRecord
		bal string
	)
	err := sc.Scan(&r.TrancatAcctID, &r.TrancatTypeCD, &r.TrancatCode, &bal)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: tcatbal scan: %w", err)
	}
	if r.TranCatBalance, err = decimal.NewFromString(bal); err != nil {
		return nil, fmt.Errorf("sqlite: tcatbal decimal %q: %w", bal, err)
	}
	return &r, nil
}

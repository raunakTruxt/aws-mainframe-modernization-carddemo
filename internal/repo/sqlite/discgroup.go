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

// DiscGroupStore is the SQLite-backed repo.DiscGroupRepository.
type DiscGroupStore struct{ db querier }

// NewDiscGroupStore returns a DiscGroupStore over db (accepts *sql.DB or *sql.Tx).
func NewDiscGroupStore(db querier) *DiscGroupStore { return &DiscGroupStore{db: db} }

const discGroupColumns = `dis_acct_group_id, dis_tran_type_cd, dis_tran_cat_cd, dis_int_rate`

func (s *DiscGroupStore) Get(ctx context.Context, groupID, typeCD string, catCD int64) (*domain.DiscGroupRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+discGroupColumns+` FROM disc_groups WHERE dis_acct_group_id = ? AND dis_tran_type_cd = ? AND dis_tran_cat_cd = ?`, groupID, typeCD, catCD)
	return scanDiscGroup(row)
}

func (s *DiscGroupStore) GetByGroup(ctx context.Context, groupID string) ([]*domain.DiscGroupRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+discGroupColumns+` FROM disc_groups WHERE dis_acct_group_id = ? ORDER BY dis_tran_type_cd, dis_tran_cat_cd`, groupID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: discgroup get-by-group: %w", err)
	}
	defer rows.Close()
	return collectDiscGroups(rows)
}

func (s *DiscGroupStore) Create(ctx context.Context, r *domain.DiscGroupRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO disc_groups (`+discGroupColumns+`) VALUES (?, ?, ?, ?)`,
		r.DisAcctGroupID, r.DisTranTypeCD, r.DisTranCatCode, r.DisIntRate.String())
	if isUniqueViolation(err) {
		return repo.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("sqlite: discgroup create: %w", err)
	}
	return nil
}

func (s *DiscGroupStore) Update(ctx context.Context, r *domain.DiscGroupRecord) error {
	res, err := s.db.ExecContext(ctx, `UPDATE disc_groups SET dis_int_rate = ? WHERE dis_acct_group_id = ? AND dis_tran_type_cd = ? AND dis_tran_cat_cd = ?`,
		r.DisIntRate.String(), r.DisAcctGroupID, r.DisTranTypeCD, r.DisTranCatCode)
	if err != nil {
		return fmt.Errorf("sqlite: discgroup update: %w", err)
	}
	return requireAffected(res)
}

func (s *DiscGroupStore) Delete(ctx context.Context, groupID, typeCD string, catCD int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM disc_groups WHERE dis_acct_group_id = ? AND dis_tran_type_cd = ? AND dis_tran_cat_cd = ?`, groupID, typeCD, catCD)
	if err != nil {
		return fmt.Errorf("sqlite: discgroup delete: %w", err)
	}
	return requireAffected(res)
}

func (s *DiscGroupStore) List(ctx context.Context) ([]*domain.DiscGroupRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+discGroupColumns+` FROM disc_groups ORDER BY dis_acct_group_id, dis_tran_type_cd, dis_tran_cat_cd`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: discgroup list: %w", err)
	}
	defer rows.Close()
	return collectDiscGroups(rows)
}

func collectDiscGroups(rows *sql.Rows) ([]*domain.DiscGroupRecord, error) {
	var out []*domain.DiscGroupRecord
	for rows.Next() {
		rec, err := scanDiscGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func scanDiscGroup(sc scanner) (*domain.DiscGroupRecord, error) {
	var (
		r    domain.DiscGroupRecord
		rate string
	)
	err := sc.Scan(&r.DisAcctGroupID, &r.DisTranTypeCD, &r.DisTranCatCode, &rate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: discgroup scan: %w", err)
	}
	if r.DisIntRate, err = decimal.NewFromString(rate); err != nil {
		return nil, fmt.Errorf("sqlite: discgroup decimal %q: %w", rate, err)
	}
	return &r, nil
}

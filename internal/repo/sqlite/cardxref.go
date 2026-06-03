package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// CardXrefStore is the SQLite-backed repo.CardXrefRepository.
type CardXrefStore struct{ db *sql.DB }

// NewCardXrefStore returns a CardXrefStore over db.
func NewCardXrefStore(db *sql.DB) *CardXrefStore { return &CardXrefStore{db: db} }

const cardXrefColumns = `xref_card_num, xref_cust_id, xref_acct_id`

func (s *CardXrefStore) Get(ctx context.Context, cardNum string) (*domain.CardXrefRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+cardXrefColumns+` FROM card_xrefs WHERE xref_card_num = ?`, cardNum)
	return scanCardXref(row)
}

func (s *CardXrefStore) GetByAccount(ctx context.Context, acctID int64) ([]*domain.CardXrefRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+cardXrefColumns+` FROM card_xrefs WHERE xref_acct_id = ? ORDER BY xref_card_num`, acctID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: cardxref get-by-account: %w", err)
	}
	defer rows.Close()
	return collectCardXrefs(rows)
}

func (s *CardXrefStore) GetByCustomer(ctx context.Context, custID int64) ([]*domain.CardXrefRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+cardXrefColumns+` FROM card_xrefs WHERE xref_cust_id = ? ORDER BY xref_card_num`, custID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: cardxref get-by-customer: %w", err)
	}
	defer rows.Close()
	return collectCardXrefs(rows)
}

func (s *CardXrefStore) Create(ctx context.Context, r *domain.CardXrefRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO card_xrefs (`+cardXrefColumns+`) VALUES (?, ?, ?)`,
		r.XrefCardNum, r.XrefCustID, r.XrefAcctID)
	if isUniqueViolation(err) {
		return repo.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("sqlite: cardxref create: %w", err)
	}
	return nil
}

func (s *CardXrefStore) Update(ctx context.Context, r *domain.CardXrefRecord) error {
	res, err := s.db.ExecContext(ctx, `UPDATE card_xrefs SET xref_cust_id = ?, xref_acct_id = ? WHERE xref_card_num = ?`,
		r.XrefCustID, r.XrefAcctID, r.XrefCardNum)
	if err != nil {
		return fmt.Errorf("sqlite: cardxref update: %w", err)
	}
	return requireAffected(res)
}

func (s *CardXrefStore) Delete(ctx context.Context, cardNum string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM card_xrefs WHERE xref_card_num = ?`, cardNum)
	if err != nil {
		return fmt.Errorf("sqlite: cardxref delete: %w", err)
	}
	return requireAffected(res)
}

func (s *CardXrefStore) Browse(ctx context.Context, startNum string, limit int) ([]*domain.CardXrefRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+cardXrefColumns+` FROM card_xrefs WHERE xref_card_num >= ? ORDER BY xref_card_num LIMIT ?`, startNum, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: cardxref browse: %w", err)
	}
	defer rows.Close()
	return collectCardXrefs(rows)
}

func collectCardXrefs(rows *sql.Rows) ([]*domain.CardXrefRecord, error) {
	var out []*domain.CardXrefRecord
	for rows.Next() {
		rec, err := scanCardXref(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func scanCardXref(sc scanner) (*domain.CardXrefRecord, error) {
	var r domain.CardXrefRecord
	err := sc.Scan(&r.XrefCardNum, &r.XrefCustID, &r.XrefAcctID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: cardxref scan: %w", err)
	}
	return &r, nil
}

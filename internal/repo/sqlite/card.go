package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// CardStore is the SQLite-backed repo.CardRepository.
type CardStore struct{ db querier }

// NewCardStore returns a CardStore over db (accepts *sql.DB or *sql.Tx).
func NewCardStore(db querier) *CardStore { return &CardStore{db: db} }

const cardColumns = `card_num, card_acct_id, card_cvv_code, card_embossed_name, card_expiration_date, card_active_status`

func (s *CardStore) Get(ctx context.Context, cardNum string) (*domain.CardRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+cardColumns+` FROM cards WHERE card_num = ?`, cardNum)
	return scanCard(row)
}

func (s *CardStore) GetByAccount(ctx context.Context, acctID int64) ([]*domain.CardRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+cardColumns+` FROM cards WHERE card_acct_id = ? ORDER BY card_num`, acctID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: card get-by-account: %w", err)
	}
	defer rows.Close()
	return collectCards(rows)
}

func (s *CardStore) Create(ctx context.Context, r *domain.CardRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO cards (`+cardColumns+`) VALUES (?, ?, ?, ?, ?, ?)`,
		r.CardNum, r.CardAcctID, r.CardCVVCode, r.CardEmbossedName, r.CardExpirationDate, r.CardActiveStatus)
	if isUniqueViolation(err) {
		return repo.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("sqlite: card create: %w", err)
	}
	return nil
}

func (s *CardStore) Update(ctx context.Context, r *domain.CardRecord) error {
	res, err := s.db.ExecContext(ctx, `UPDATE cards SET card_acct_id = ?, card_cvv_code = ?, card_embossed_name = ?, card_expiration_date = ?, card_active_status = ? WHERE card_num = ?`,
		r.CardAcctID, r.CardCVVCode, r.CardEmbossedName, r.CardExpirationDate, r.CardActiveStatus, r.CardNum)
	if err != nil {
		return fmt.Errorf("sqlite: card update: %w", err)
	}
	return requireAffected(res)
}

func (s *CardStore) Delete(ctx context.Context, cardNum string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM cards WHERE card_num = ?`, cardNum)
	if err != nil {
		return fmt.Errorf("sqlite: card delete: %w", err)
	}
	return requireAffected(res)
}

func (s *CardStore) Browse(ctx context.Context, startNum string, limit int) ([]*domain.CardRecord, error) {
	if limit <= 0 {
		limit = -1
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+cardColumns+` FROM cards WHERE card_num >= ? ORDER BY card_num LIMIT ?`, startNum, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: card browse: %w", err)
	}
	defer rows.Close()
	return collectCards(rows)
}

func collectCards(rows *sql.Rows) ([]*domain.CardRecord, error) {
	var out []*domain.CardRecord
	for rows.Next() {
		rec, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func scanCard(sc scanner) (*domain.CardRecord, error) {
	var r domain.CardRecord
	err := sc.Scan(&r.CardNum, &r.CardAcctID, &r.CardCVVCode, &r.CardEmbossedName, &r.CardExpirationDate, &r.CardActiveStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: card scan: %w", err)
	}
	return &r, nil
}

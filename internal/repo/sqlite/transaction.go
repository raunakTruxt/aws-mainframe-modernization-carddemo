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

// TransactionStore is the SQLite-backed repo.TransactionRepository.
type TransactionStore struct{ db querier }

// NewTransactionStore returns a TransactionStore over db (accepts *sql.DB or *sql.Tx).
func NewTransactionStore(db querier) *TransactionStore { return &TransactionStore{db: db} }

const transactionColumns = `tran_id, tran_type_code, tran_cat_code, tran_source, tran_desc, tran_amt, tran_merchant_id, tran_merchant_name, tran_merchant_city, tran_merchant_zip, tran_card_num, tran_orig_ts, tran_proc_ts`

func (s *TransactionStore) Get(ctx context.Context, tranID string) (*domain.TransactionRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE tran_id = ?`, tranID)
	return scanTransaction(row)
}

func (s *TransactionStore) GetByCard(ctx context.Context, cardNum string, limit int) ([]*domain.TransactionRecord, error) {
	if limit <= 0 {
		limit = -1
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE tran_card_num = ? ORDER BY tran_id LIMIT ?`, cardNum, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: transaction get-by-card: %w", err)
	}
	defer rows.Close()
	return collectTransactions(rows)
}

func (s *TransactionStore) Create(ctx context.Context, r *domain.TransactionRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO transactions (`+transactionColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.TranID, r.TranTypeCode, r.TranCatCode, r.TranSource, r.TranDesc, r.TranAmt.String(), r.TranMerchantID,
		r.TranMerchantName, r.TranMerchantCity, r.TranMerchantZip, r.TranCardNum, r.TranOrigTS, r.TranProcTS)
	if isUniqueViolation(err) {
		return repo.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("sqlite: transaction create: %w", err)
	}
	return nil
}

func (s *TransactionStore) Update(ctx context.Context, r *domain.TransactionRecord) error {
	res, err := s.db.ExecContext(ctx, `UPDATE transactions SET tran_type_code = ?, tran_cat_code = ?, tran_source = ?, tran_desc = ?, tran_amt = ?, tran_merchant_id = ?, tran_merchant_name = ?, tran_merchant_city = ?, tran_merchant_zip = ?, tran_card_num = ?, tran_orig_ts = ?, tran_proc_ts = ? WHERE tran_id = ?`,
		r.TranTypeCode, r.TranCatCode, r.TranSource, r.TranDesc, r.TranAmt.String(), r.TranMerchantID,
		r.TranMerchantName, r.TranMerchantCity, r.TranMerchantZip, r.TranCardNum, r.TranOrigTS, r.TranProcTS, r.TranID)
	if err != nil {
		return fmt.Errorf("sqlite: transaction update: %w", err)
	}
	return requireAffected(res)
}

func (s *TransactionStore) Delete(ctx context.Context, tranID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM transactions WHERE tran_id = ?`, tranID)
	if err != nil {
		return fmt.Errorf("sqlite: transaction delete: %w", err)
	}
	return requireAffected(res)
}

func (s *TransactionStore) Browse(ctx context.Context, startID string, limit int) ([]*domain.TransactionRecord, error) {
	if limit <= 0 {
		limit = -1
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE tran_id >= ? ORDER BY tran_id LIMIT ?`, startID, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: transaction browse: %w", err)
	}
	defer rows.Close()
	return collectTransactions(rows)
}

func collectTransactions(rows *sql.Rows) ([]*domain.TransactionRecord, error) {
	var out []*domain.TransactionRecord
	for rows.Next() {
		rec, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func scanTransaction(sc scanner) (*domain.TransactionRecord, error) {
	var (
		r   domain.TransactionRecord
		amt string
	)
	err := sc.Scan(&r.TranID, &r.TranTypeCode, &r.TranCatCode, &r.TranSource, &r.TranDesc, &amt, &r.TranMerchantID,
		&r.TranMerchantName, &r.TranMerchantCity, &r.TranMerchantZip, &r.TranCardNum, &r.TranOrigTS, &r.TranProcTS)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: transaction scan: %w", err)
	}
	if r.TranAmt, err = decimal.NewFromString(amt); err != nil {
		return nil, fmt.Errorf("sqlite: transaction decimal %q: %w", amt, err)
	}
	return &r, nil
}

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

// AccountStore is the SQLite-backed repo.AccountRepository.
type AccountStore struct{ db *sql.DB }

// NewAccountStore returns an AccountStore over db.
func NewAccountStore(db *sql.DB) *AccountStore { return &AccountStore{db: db} }

const accountColumns = `acct_id, acct_active_status, acct_curr_bal, acct_credit_limit, acct_cash_credit_limit, acct_open_date, acct_expiration_date, acct_reissue_date, acct_curr_cyc_credit, acct_curr_cyc_debit, acct_addr_zip, acct_group_id`

func (s *AccountStore) Get(ctx context.Context, acctID int64) (*domain.AccountRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+accountColumns+` FROM accounts WHERE acct_id = ?`, acctID)
	return scanAccount(row)
}

func (s *AccountStore) Create(ctx context.Context, r *domain.AccountRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO accounts (`+accountColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.AcctID, r.AcctActiveStatus, r.AcctCurrBal.String(), r.AcctCreditLimit.String(), r.AcctCashCreditLimit.String(),
		r.AcctOpenDate, r.AcctExpirationDate, r.AcctReissueDate, r.AcctCurrCycCredit.String(), r.AcctCurrCycDebit.String(),
		r.AcctAddrZip, r.AcctGroupID)
	if isUniqueViolation(err) {
		return repo.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("sqlite: account create: %w", err)
	}
	return nil
}

func (s *AccountStore) Update(ctx context.Context, r *domain.AccountRecord) error {
	res, err := s.db.ExecContext(ctx, `UPDATE accounts SET acct_active_status = ?, acct_curr_bal = ?, acct_credit_limit = ?, acct_cash_credit_limit = ?, acct_open_date = ?, acct_expiration_date = ?, acct_reissue_date = ?, acct_curr_cyc_credit = ?, acct_curr_cyc_debit = ?, acct_addr_zip = ?, acct_group_id = ? WHERE acct_id = ?`,
		r.AcctActiveStatus, r.AcctCurrBal.String(), r.AcctCreditLimit.String(), r.AcctCashCreditLimit.String(),
		r.AcctOpenDate, r.AcctExpirationDate, r.AcctReissueDate, r.AcctCurrCycCredit.String(), r.AcctCurrCycDebit.String(),
		r.AcctAddrZip, r.AcctGroupID, r.AcctID)
	if err != nil {
		return fmt.Errorf("sqlite: account update: %w", err)
	}
	return requireAffected(res)
}

func (s *AccountStore) Delete(ctx context.Context, acctID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM accounts WHERE acct_id = ?`, acctID)
	if err != nil {
		return fmt.Errorf("sqlite: account delete: %w", err)
	}
	return requireAffected(res)
}

func (s *AccountStore) Browse(ctx context.Context, startID int64, limit int) ([]*domain.AccountRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+accountColumns+` FROM accounts WHERE acct_id >= ? ORDER BY acct_id LIMIT ?`, startID, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: account browse: %w", err)
	}
	defer rows.Close()

	var out []*domain.AccountRecord
	for rows.Next() {
		rec, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func scanAccount(sc scanner) (*domain.AccountRecord, error) {
	var (
		r                                             domain.AccountRecord
		currBal, creditLimit, cashLimit, cycCr, cycDb string
	)
	err := sc.Scan(&r.AcctID, &r.AcctActiveStatus, &currBal, &creditLimit, &cashLimit,
		&r.AcctOpenDate, &r.AcctExpirationDate, &r.AcctReissueDate, &cycCr, &cycDb,
		&r.AcctAddrZip, &r.AcctGroupID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: account scan: %w", err)
	}
	for _, d := range []struct {
		dst *decimal.Decimal
		raw string
	}{
		{&r.AcctCurrBal, currBal},
		{&r.AcctCreditLimit, creditLimit},
		{&r.AcctCashCreditLimit, cashLimit},
		{&r.AcctCurrCycCredit, cycCr},
		{&r.AcctCurrCycDebit, cycDb},
	} {
		if *d.dst, err = decimal.NewFromString(d.raw); err != nil {
			return nil, fmt.Errorf("sqlite: account decimal %q: %w", d.raw, err)
		}
	}
	return &r, nil
}

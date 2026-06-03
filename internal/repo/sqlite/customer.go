package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

// CustomerStore is the SQLite-backed repo.CustomerRepository.
type CustomerStore struct{ db querier }

// NewCustomerStore returns a CustomerStore over db (accepts *sql.DB or *sql.Tx).
func NewCustomerStore(db querier) *CustomerStore { return &CustomerStore{db: db} }

const customerColumns = `cust_id, cust_first_name, cust_middle_name, cust_last_name, cust_addr_line_1, cust_addr_line_2, cust_addr_line_3, cust_addr_state_code, cust_addr_country_code, cust_addr_zip, cust_phone_num_1, cust_phone_num_2, cust_ssn, cust_govt_issued_id, cust_dob, cust_eft_account_id, cust_pri_card_holder_ind, cust_fico_credit_score`

func (s *CustomerStore) Get(ctx context.Context, custID int64) (*domain.CustomerRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+customerColumns+` FROM customers WHERE cust_id = ?`, custID)
	return scanCustomer(row)
}

func (s *CustomerStore) Create(ctx context.Context, r *domain.CustomerRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO customers (`+customerColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.CustID, r.CustFirstName, r.CustMiddleName, r.CustLastName, r.CustAddrLine1, r.CustAddrLine2, r.CustAddrLine3,
		r.CustAddrStateCode, r.CustAddrCountryCode, r.CustAddrZip, r.CustPhoneNum1, r.CustPhoneNum2, r.CustSSN,
		r.CustGovtIssuedID, r.CustDOB, r.CustEFTAccountID, r.CustPriCardHolderInd, r.CustFICOCreditScore)
	if isUniqueViolation(err) {
		return repo.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("sqlite: customer create: %w", err)
	}
	return nil
}

func (s *CustomerStore) Update(ctx context.Context, r *domain.CustomerRecord) error {
	res, err := s.db.ExecContext(ctx, `UPDATE customers SET cust_first_name = ?, cust_middle_name = ?, cust_last_name = ?, cust_addr_line_1 = ?, cust_addr_line_2 = ?, cust_addr_line_3 = ?, cust_addr_state_code = ?, cust_addr_country_code = ?, cust_addr_zip = ?, cust_phone_num_1 = ?, cust_phone_num_2 = ?, cust_ssn = ?, cust_govt_issued_id = ?, cust_dob = ?, cust_eft_account_id = ?, cust_pri_card_holder_ind = ?, cust_fico_credit_score = ? WHERE cust_id = ?`,
		r.CustFirstName, r.CustMiddleName, r.CustLastName, r.CustAddrLine1, r.CustAddrLine2, r.CustAddrLine3,
		r.CustAddrStateCode, r.CustAddrCountryCode, r.CustAddrZip, r.CustPhoneNum1, r.CustPhoneNum2, r.CustSSN,
		r.CustGovtIssuedID, r.CustDOB, r.CustEFTAccountID, r.CustPriCardHolderInd, r.CustFICOCreditScore, r.CustID)
	if err != nil {
		return fmt.Errorf("sqlite: customer update: %w", err)
	}
	return requireAffected(res)
}

func (s *CustomerStore) Delete(ctx context.Context, custID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM customers WHERE cust_id = ?`, custID)
	if err != nil {
		return fmt.Errorf("sqlite: customer delete: %w", err)
	}
	return requireAffected(res)
}

func (s *CustomerStore) Browse(ctx context.Context, startID int64, limit int) ([]*domain.CustomerRecord, error) {
	if limit <= 0 {
		limit = -1
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+customerColumns+` FROM customers WHERE cust_id >= ? ORDER BY cust_id LIMIT ?`, startID, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite: customer browse: %w", err)
	}
	defer rows.Close()

	var out []*domain.CustomerRecord
	for rows.Next() {
		rec, err := scanCustomer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func scanCustomer(sc scanner) (*domain.CustomerRecord, error) {
	var r domain.CustomerRecord
	err := sc.Scan(&r.CustID, &r.CustFirstName, &r.CustMiddleName, &r.CustLastName, &r.CustAddrLine1, &r.CustAddrLine2,
		&r.CustAddrLine3, &r.CustAddrStateCode, &r.CustAddrCountryCode, &r.CustAddrZip, &r.CustPhoneNum1, &r.CustPhoneNum2,
		&r.CustSSN, &r.CustGovtIssuedID, &r.CustDOB, &r.CustEFTAccountID, &r.CustPriCardHolderInd, &r.CustFICOCreditScore)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, repo.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: customer scan: %w", err)
	}
	return &r, nil
}

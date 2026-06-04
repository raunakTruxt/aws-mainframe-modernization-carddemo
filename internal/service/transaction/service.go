// Package transaction is the Go port of the CardDemo online transaction
// programs: COTRN00C (list), COTRN01C (view), COTRN02C (add), COBIL00C (pay).
package transaction

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/shopspring/decimal"
)

var (
	// ErrInvalidAmount is returned when the transaction amount is zero.
	ErrInvalidAmount = errors.New("transaction: amount must not be zero")
	// ErrAmountNegativeOrZero is returned when a payment amount is not positive.
	ErrAmountNegativeOrZero = errors.New("transaction: payment amount must be positive")
	// ErrInvalidTranType is returned when the type code does not exist.
	ErrInvalidTranType = errors.New("transaction: invalid transaction type code")
	// ErrInvalidTranCat is returned when the type/category combination does not exist.
	ErrInvalidTranCat = errors.New("transaction: invalid transaction category")
	// ErrAccountNotFound is returned when the linked account is not found.
	ErrAccountNotFound = errors.New("transaction: account not found")
	// ErrAccountClosed is returned when the account is not active.
	ErrAccountClosed = errors.New("transaction: account is closed")
	// ErrCardNotFound is returned when the card is not found.
	ErrCardNotFound = errors.New("transaction: card not found")
	// ErrCardInactive is returned when the card is not active.
	ErrCardInactive = errors.New("transaction: card is inactive")
	// ErrCreditLimitExceeded is returned when the transaction would exceed the credit limit.
	ErrCreditLimitExceeded = errors.New("transaction: amount would exceed credit limit")
	// ErrPaymentExceedsBalance is returned when the payment exceeds the current balance.
	ErrPaymentExceedsBalance = errors.New("transaction: payment amount exceeds current balance")
	// ErrZeroBalance is returned when a bill payment is attempted with no balance owing.
	ErrZeroBalance = errors.New("transaction: no balance owing; nothing to pay")
)

// Transactor wraps a Create+Update write pair in a database transaction.
// The fn callback receives tx-scoped TransactionRepository and
// AccountRepository; changes are committed on success and rolled back on error.
//
// This restores the CICS syncpoint atomicity that the legacy COTRN02C and
// COBIL00C relied on: without it, a process crash after Create but before
// Update would leave an orphan transaction row with a stale account balance.
//
// The sqlite package provides a concrete implementation (sqlite.NewTransactor).
type Transactor interface {
	Do(ctx context.Context, fn func(txns repo.TransactionRepository, accts repo.AccountRepository) error) error
}

// AddRequest carries the caller-supplied fields for creating a new transaction
// (COTRN02C). The TranCardNum field identifies both the card and — via lookup —
// the account whose balance is updated.
type AddRequest struct {
	TranTypeCode     string
	TranCatCode      int64
	TranSource       string
	TranDesc         string
	TranAmt          decimal.Decimal
	TranMerchantID   int64
	TranMerchantName string
	TranMerchantCity string
	TranMerchantZip  string
	TranCardNum      string
}

// PayRequest carries the caller-supplied fields for a bill payment (COBIL00C).
type PayRequest struct {
	CardNum string
	Amt     decimal.Decimal
}

// Service orchestrates the transaction and bill-payment business logic.
type Service struct {
	Transact  Transactor
	Txns      repo.TransactionRepository
	Accounts  repo.AccountRepository
	Cards     repo.CardRepository
	TranTypes repo.TranTypeRepository
	TranCats  repo.TranCatRepository

	// PayTranType and PayTranCatCode identify the payment transaction type used
	// by COBIL00C (hardcoded '02' / 0 in the legacy system).
	PayTranType    string
	PayTranCatCode int64

	// Now is the clock function; defaults to time.Now for production.
	Now func() time.Time
}

// New returns a Service with sensible defaults (payment type '02', cat 0).
func New(
	transact Transactor,
	txns repo.TransactionRepository,
	accounts repo.AccountRepository,
	cards repo.CardRepository,
	tranTypes repo.TranTypeRepository,
	tranCats repo.TranCatRepository,
) *Service {
	return &Service{
		Transact:       transact,
		Txns:           txns,
		Accounts:       accounts,
		Cards:          cards,
		TranTypes:      tranTypes,
		TranCats:       tranCats,
		PayTranType:    "02",
		PayTranCatCode: 0,
		Now:            time.Now,
	}
}

// List returns transactions for cardNum (COTRN00C paged browse by card AIX).
// If cardNum is empty it browses the primary index from startID.
// limit <= 0 returns all matching rows.
func (s *Service) List(ctx context.Context, cardNum, startID string, limit int) ([]*domain.TransactionRecord, error) {
	if cardNum != "" {
		return s.Txns.GetByCard(ctx, cardNum, limit)
	}
	return s.Txns.Browse(ctx, startID, limit)
}

// Get returns a single transaction by ID (COTRN01C).
func (s *Service) Get(ctx context.Context, tranID string) (*domain.TransactionRecord, error) {
	return s.Txns.Get(ctx, tranID)
}

// Add validates and creates a new debit/credit transaction (COTRN02C).
//
// Validation mirrors COTRN02C VALIDATE-INPUT-DATA-FIELDS:
//   - Amount must not be zero.
//   - Type code must exist in TRANTYPE (TranTypeRepository).
//   - Type+category must exist in TRANCATG (TranCatRepository).
//   - Card must exist and be active (CardActiveStatus == "Y").
//   - Account linked to the card must exist and be active (AcctActiveStatus == "Y").
//   - AcctCurrBal + TranAmt must not exceed AcctCreditLimit (credit limit guard).
//
// The TRAN-ID is allocated and both the transaction insert and account balance
// update are performed inside a single database transaction (Transactor.Do),
// reproducing the CICS syncpoint atomicity of the original COTRN02C.
func (s *Service) Add(ctx context.Context, req AddRequest) (*domain.TransactionRecord, error) {
	if req.TranAmt.IsZero() {
		return nil, ErrInvalidAmount
	}

	if _, err := s.TranTypes.Get(ctx, req.TranTypeCode); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrInvalidTranType
		}
		return nil, fmt.Errorf("transaction add: lookup tran type: %w", err)
	}

	if _, err := s.TranCats.Get(ctx, req.TranTypeCode, req.TranCatCode); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrInvalidTranCat
		}
		return nil, fmt.Errorf("transaction add: lookup tran cat: %w", err)
	}

	card, err := s.Cards.Get(ctx, req.TranCardNum)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrCardNotFound
		}
		return nil, fmt.Errorf("transaction add: lookup card: %w", err)
	}
	if !strings.EqualFold(card.CardActiveStatus, "Y") {
		return nil, ErrCardInactive
	}

	// Pre-flight check outside the transaction for early exit with a clean error.
	acct, err := s.Accounts.Get(ctx, card.CardAcctID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("transaction add: lookup account: %w", err)
	}
	if !strings.EqualFold(acct.AcctActiveStatus, "Y") {
		return nil, ErrAccountClosed
	}
	if acct.AcctCurrBal.Add(req.TranAmt).GreaterThan(acct.AcctCreditLimit) {
		return nil, ErrCreditLimitExceeded
	}

	now := s.Now()
	ts := now.UTC().Format("2006-01-02-15.04.05.000000")

	var result *domain.TransactionRecord
	txErr := s.Transact.Do(ctx, func(txns repo.TransactionRepository, accts repo.AccountRepository) error {
		// Allocate a collision-free TRAN-ID inside the transaction.
		nextID, idErr := txns.NextID(ctx)
		if idErr != nil {
			return fmt.Errorf("next id: %w", idErr)
		}

		// Re-read the account under the transaction lock; recompute balance to
		// guard against a concurrent update between the pre-flight check above
		// and now.
		txAcct, aErr := accts.Get(ctx, card.CardAcctID)
		if aErr != nil {
			return fmt.Errorf("re-read account: %w", aErr)
		}
		newBal := txAcct.AcctCurrBal.Add(req.TranAmt)
		if newBal.GreaterThan(txAcct.AcctCreditLimit) {
			return ErrCreditLimitExceeded
		}

		result = &domain.TransactionRecord{
			TranID:           nextID,
			TranTypeCode:     req.TranTypeCode,
			TranCatCode:      req.TranCatCode,
			TranSource:       req.TranSource,
			TranDesc:         req.TranDesc,
			TranAmt:          req.TranAmt,
			TranMerchantID:   req.TranMerchantID,
			TranMerchantName: req.TranMerchantName,
			TranMerchantCity: req.TranMerchantCity,
			TranMerchantZip:  req.TranMerchantZip,
			TranCardNum:      req.TranCardNum,
			TranOrigTS:       ts,
			TranProcTS:       ts,
		}

		if createErr := txns.Create(ctx, result); createErr != nil {
			return fmt.Errorf("create: %w", createErr)
		}
		txAcct.AcctCurrBal = newBal
		return accts.Update(ctx, txAcct)
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

// Pay validates and processes a bill payment (COBIL00C).
//
// Validation mirrors COBIL00C:
//   - Payment amount must be positive.
//   - The current balance must be positive (no payment when nothing is owed).
//   - Payment amount must not exceed the current outstanding balance.
//
// The TRAN-ID is allocated and both the payment insert and account balance
// decrement are performed inside a single database transaction (Transactor.Do).
func (s *Service) Pay(ctx context.Context, req PayRequest) (*domain.TransactionRecord, error) {
	if req.Amt.LessThanOrEqual(decimal.Zero) {
		return nil, ErrAmountNegativeOrZero
	}

	card, err := s.Cards.Get(ctx, req.CardNum)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrCardNotFound
		}
		return nil, fmt.Errorf("transaction pay: lookup card: %w", err)
	}

	// Pre-flight check outside the transaction for early exit with a clean error.
	acct, err := s.Accounts.Get(ctx, card.CardAcctID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("transaction pay: lookup account: %w", err)
	}
	if acct.AcctCurrBal.LessThanOrEqual(decimal.Zero) {
		return nil, ErrZeroBalance
	}
	if req.Amt.GreaterThan(acct.AcctCurrBal) {
		return nil, ErrPaymentExceedsBalance
	}

	now := s.Now()
	ts := now.UTC().Format("2006-01-02-15.04.05.000000")

	var result *domain.TransactionRecord
	txErr := s.Transact.Do(ctx, func(txns repo.TransactionRepository, accts repo.AccountRepository) error {
		nextID, idErr := txns.NextID(ctx)
		if idErr != nil {
			return fmt.Errorf("next id: %w", idErr)
		}

		// Re-read account under the transaction lock.
		txAcct, aErr := accts.Get(ctx, card.CardAcctID)
		if aErr != nil {
			return fmt.Errorf("re-read account: %w", aErr)
		}
		if txAcct.AcctCurrBal.LessThanOrEqual(decimal.Zero) {
			return ErrZeroBalance
		}
		if req.Amt.GreaterThan(txAcct.AcctCurrBal) {
			return ErrPaymentExceedsBalance
		}

		result = &domain.TransactionRecord{
			TranID:           nextID,
			TranTypeCode:     s.PayTranType,
			TranCatCode:      s.PayTranCatCode,
			TranSource:       "ONLINE",
			TranDesc:         "BILL PAYMENT - ONLINE",
			TranAmt:          req.Amt,
			TranMerchantID:   0,
			TranMerchantName: "BILL PAYMENT",
			TranMerchantCity: "",
			TranMerchantZip:  "",
			TranCardNum:      req.CardNum,
			TranOrigTS:       ts,
			TranProcTS:       ts,
		}

		if createErr := txns.Create(ctx, result); createErr != nil {
			return fmt.Errorf("create: %w", createErr)
		}
		txAcct.AcctCurrBal = txAcct.AcctCurrBal.Sub(req.Amt)
		return accts.Update(ctx, txAcct)
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

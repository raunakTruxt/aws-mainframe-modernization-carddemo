// Package transaction is the Go port of the CardDemo online transaction
// programs: COTRN00C (list), COTRN01C (view), COTRN02C (add), COBIL00C (pay).
package transaction

import (
	"context"
	"crypto/rand"
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
	txns repo.TransactionRepository,
	accounts repo.AccountRepository,
	cards repo.CardRepository,
	tranTypes repo.TranTypeRepository,
	tranCats repo.TranCatRepository,
) *Service {
	return &Service{
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
// On success, the account balance is updated and the new record is returned.
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

	// Credit limit guard: for debit transactions, new balance must not exceed limit.
	newBal := acct.AcctCurrBal.Add(req.TranAmt)
	if newBal.GreaterThan(acct.AcctCreditLimit) {
		return nil, ErrCreditLimitExceeded
	}

	now := s.Now()
	ts := now.UTC().Format("2006-01-02-15.04.05.000000")
	rec := &domain.TransactionRecord{
		TranID:           generateTranID(now),
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

	if err := s.Txns.Create(ctx, rec); err != nil {
		return nil, fmt.Errorf("transaction add: create: %w", err)
	}

	acct.AcctCurrBal = newBal
	if err := s.Accounts.Update(ctx, acct); err != nil {
		return nil, fmt.Errorf("transaction add: update account balance: %w", err)
	}

	return rec, nil
}

// Pay validates and processes a bill payment (COBIL00C).
//
// Validation mirrors COBIL00C:
//   - Payment amount must be positive.
//   - The current balance must be positive (no payment when nothing is owed).
//   - Payment amount must not exceed the current outstanding balance.
//
// A credit transaction record is created with type s.PayTranType and the
// account balance is decremented by the payment amount.
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
	rec := &domain.TransactionRecord{
		TranID:           generateTranID(now),
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

	if err := s.Txns.Create(ctx, rec); err != nil {
		return nil, fmt.Errorf("transaction pay: create: %w", err)
	}

	acct.AcctCurrBal = acct.AcctCurrBal.Sub(req.Amt)
	if err := s.Accounts.Update(ctx, acct); err != nil {
		return nil, fmt.Errorf("transaction pay: update account balance: %w", err)
	}

	return rec, nil
}

// generateTranID produces a 16-character legacy-format TRAN-ID:
//
//	YYYYMMDDHHMMSS (14 chars, UTC) + 2 hex digits from crypto/rand.
//
// The timestamp prefix makes IDs sortable by creation time; the random suffix
// provides uniqueness within the same second.
func generateTranID(now time.Time) string {
	suffix := make([]byte, 1)
	if _, err := rand.Read(suffix); err != nil {
		suffix[0] = 0
	}
	return fmt.Sprintf("%s%02X", now.UTC().Format("20060102150405"), suffix[0])
}

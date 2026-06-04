package transaction_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	svc "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/transaction"
	"github.com/shopspring/decimal"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newService(t *testing.T, db *sql.DB) *svc.Service {
	t.Helper()
	fixedNow := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	s := svc.New(
		sqlite.NewTransactionStore(db),
		sqlite.NewAccountStore(db),
		sqlite.NewCardStore(db),
		sqlite.NewTranTypeStore(db),
		sqlite.NewTranCatStore(db),
	)
	s.Now = func() time.Time { return fixedNow }
	return s
}

func seedBasicData(t *testing.T, db *sql.DB) (cardNum string, acctID int64) {
	t.Helper()
	ctx := context.Background()
	cardNum = "4111111111111111"
	acctID = 11111111111

	acct := &domain.AccountRecord{
		AcctID:           acctID,
		AcctActiveStatus: "Y",
		AcctCurrBal:      decimal.RequireFromString("500.00"),
		AcctCreditLimit:  decimal.RequireFromString("1000.00"),
		AcctOpenDate:     "2020-01-01",
		AcctExpirationDate: "2030-01-01",
		AcctReissueDate:  "2025-01-01",
		AcctCurrCycCredit: decimal.Zero,
		AcctCurrCycDebit:  decimal.Zero,
		AcctAddrZip:      "90210",
		AcctGroupID:      "GRP001",
		AcctCashCreditLimit: decimal.RequireFromString("200.00"),
	}
	if err := sqlite.NewAccountStore(db).Create(ctx, acct); err != nil {
		t.Fatalf("seed account: %v", err)
	}

	card := &domain.CardRecord{
		CardNum:            cardNum,
		CardAcctID:         acctID,
		CardCVVCode:        123,
		CardEmbossedName:   "USER0001",
		CardExpirationDate: "2030-01-01",
		CardActiveStatus:   "Y",
	}
	if err := sqlite.NewCardStore(db).Create(ctx, card); err != nil {
		t.Fatalf("seed card: %v", err)
	}

	tt := &domain.TranTypeRecord{TranType: "01", TranTypeDesc: "Purchase"}
	if err := sqlite.NewTranTypeStore(db).Create(ctx, tt); err != nil {
		t.Fatalf("seed tran type: %v", err)
	}

	tc := &domain.TranCatRecord{TranTypeCode: "01", TranCatCode: 1000, TranCatTypeDesc: "Retail"}
	if err := sqlite.NewTranCatStore(db).Create(ctx, tc); err != nil {
		t.Fatalf("seed tran cat: %v", err)
	}

	return cardNum, acctID
}

func TestList(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, _ := seedBasicData(t, db)

	// Create two transactions for the card.
	for _, req := range []svc.AddRequest{
		{
			TranTypeCode: "01", TranCatCode: 1000, TranSource: "POS",
			TranDesc: "PURCHASE 1", TranAmt: decimal.RequireFromString("10.00"),
			TranCardNum: cardNum,
		},
		{
			TranTypeCode: "01", TranCatCode: 1000, TranSource: "POS",
			TranDesc: "PURCHASE 2", TranAmt: decimal.RequireFromString("20.00"),
			TranCardNum: cardNum,
		},
	} {
		if _, err := s.Add(ctx, req); err != nil {
			t.Fatalf("add: %v", err)
		}
	}

	// List by card — should return both.
	txns, err := s.List(ctx, cardNum, "", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(txns) != 2 {
		t.Fatalf("want 2 txns, got %d", len(txns))
	}
	for _, tx := range txns {
		if tx.TranCardNum != cardNum {
			t.Fatalf("wrong card num in result: %s", tx.TranCardNum)
		}
	}

	// List without card — browses all from start.
	all, err := s.List(ctx, "", "", 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 total, got %d", len(all))
	}
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, _ := seedBasicData(t, db)

	added, err := s.Add(ctx, svc.AddRequest{
		TranTypeCode: "01", TranCatCode: 1000, TranSource: "POS",
		TranDesc: "PURCHASE", TranAmt: decimal.RequireFromString("10.00"),
		TranCardNum: cardNum,
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	got, err := s.Get(ctx, added.TranID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TranID != added.TranID {
		t.Fatalf("want id %s, got %s", added.TranID, got.TranID)
	}
	if !got.TranAmt.Equal(added.TranAmt) {
		t.Fatalf("want amt %s, got %s", added.TranAmt, got.TranAmt)
	}
}

func TestAdd_ValidTransaction(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, acctID := seedBasicData(t, db)

	req := svc.AddRequest{
		TranTypeCode:     "01",
		TranCatCode:      1000,
		TranSource:       "POS",
		TranDesc:         "COFFEE",
		TranAmt:          decimal.RequireFromString("5.00"),
		TranMerchantName: "ACME CAFE",
		TranMerchantCity: "CITYVILLE",
		TranMerchantZip:  "90210",
		TranCardNum:      cardNum,
	}
	rec, err := s.Add(ctx, req)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(rec.TranID) != 16 {
		t.Fatalf("tran id length: want 16, got %d", len(rec.TranID))
	}
	if !rec.TranAmt.Equal(req.TranAmt) {
		t.Fatalf("amount mismatch: want %s, got %s", req.TranAmt, rec.TranAmt)
	}

	// Account balance must be updated.
	acct, err := sqlite.NewAccountStore(db).Get(ctx, acctID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	// Original 500.00 + 5.00 = 505.00
	if !acct.AcctCurrBal.Equal(decimal.RequireFromString("505.00")) {
		t.Fatalf("balance: want 505.00, got %s", acct.AcctCurrBal)
	}
}

func TestAdd_ZeroAmount(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, _ := seedBasicData(t, db)

	_, err := s.Add(ctx, svc.AddRequest{
		TranTypeCode: "01", TranCatCode: 1000, TranCardNum: cardNum,
		TranAmt: decimal.Zero,
	})
	if !errors.Is(err, svc.ErrInvalidAmount) {
		t.Fatalf("want ErrInvalidAmount, got %v", err)
	}
}

func TestAdd_InvalidTranType(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, _ := seedBasicData(t, db)

	_, err := s.Add(ctx, svc.AddRequest{
		TranTypeCode: "ZZ", TranCatCode: 1000, TranCardNum: cardNum,
		TranAmt: decimal.RequireFromString("10.00"),
	})
	if !errors.Is(err, svc.ErrInvalidTranType) {
		t.Fatalf("want ErrInvalidTranType, got %v", err)
	}
}

func TestAdd_InvalidTranCat(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, _ := seedBasicData(t, db)

	_, err := s.Add(ctx, svc.AddRequest{
		TranTypeCode: "01", TranCatCode: 9999, TranCardNum: cardNum,
		TranAmt: decimal.RequireFromString("10.00"),
	})
	if !errors.Is(err, svc.ErrInvalidTranCat) {
		t.Fatalf("want ErrInvalidTranCat, got %v", err)
	}
}

func TestAdd_CardNotFound(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	seedBasicData(t, db)

	_, err := s.Add(ctx, svc.AddRequest{
		TranTypeCode: "01", TranCatCode: 1000, TranCardNum: "9999999999999999",
		TranAmt: decimal.RequireFromString("10.00"),
	})
	if !errors.Is(err, svc.ErrCardNotFound) {
		t.Fatalf("want ErrCardNotFound, got %v", err)
	}
}

func TestAdd_CardInactive(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, _ := seedBasicData(t, db)

	// Deactivate card.
	cardStore := sqlite.NewCardStore(db)
	card, _ := cardStore.Get(ctx, cardNum)
	card.CardActiveStatus = "N"
	if err := cardStore.Update(ctx, card); err != nil {
		t.Fatalf("update card: %v", err)
	}

	_, err := s.Add(ctx, svc.AddRequest{
		TranTypeCode: "01", TranCatCode: 1000, TranCardNum: cardNum,
		TranAmt: decimal.RequireFromString("10.00"),
	})
	if !errors.Is(err, svc.ErrCardInactive) {
		t.Fatalf("want ErrCardInactive, got %v", err)
	}
}

func TestAdd_AccountClosed(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, acctID := seedBasicData(t, db)

	acctStore := sqlite.NewAccountStore(db)
	acct, _ := acctStore.Get(ctx, acctID)
	acct.AcctActiveStatus = "N"
	if err := acctStore.Update(ctx, acct); err != nil {
		t.Fatalf("update account: %v", err)
	}

	_, err := s.Add(ctx, svc.AddRequest{
		TranTypeCode: "01", TranCatCode: 1000, TranCardNum: cardNum,
		TranAmt: decimal.RequireFromString("10.00"),
	})
	if !errors.Is(err, svc.ErrAccountClosed) {
		t.Fatalf("want ErrAccountClosed, got %v", err)
	}
}

func TestAdd_ExceedsCreditLimit(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, _ := seedBasicData(t, db)

	// Current balance is 500.00, limit is 1000.00. Try to add 600.00 → total 1100.00 > 1000.00.
	_, err := s.Add(ctx, svc.AddRequest{
		TranTypeCode: "01", TranCatCode: 1000, TranCardNum: cardNum,
		TranAmt: decimal.RequireFromString("600.00"),
	})
	if !errors.Is(err, svc.ErrCreditLimitExceeded) {
		t.Fatalf("want ErrCreditLimitExceeded, got %v", err)
	}
}

func TestPay_Valid(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, acctID := seedBasicData(t, db)

	// Pay 100.00 against 500.00 balance.
	rec, err := s.Pay(ctx, svc.PayRequest{
		CardNum: cardNum,
		Amt:     decimal.RequireFromString("100.00"),
	})
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if rec.TranDesc != "BILL PAYMENT - ONLINE" {
		t.Fatalf("wrong desc: %s", rec.TranDesc)
	}
	if !rec.TranAmt.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("wrong amt: %s", rec.TranAmt)
	}
	if rec.TranTypeCode != "02" {
		t.Fatalf("wrong type code: %s", rec.TranTypeCode)
	}

	// Balance must drop from 500.00 to 400.00.
	acct, err := sqlite.NewAccountStore(db).Get(ctx, acctID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if !acct.AcctCurrBal.Equal(decimal.RequireFromString("400.00")) {
		t.Fatalf("balance: want 400.00, got %s", acct.AcctCurrBal)
	}

	// Exactly one transaction record created.
	txns, err := s.List(ctx, cardNum, "", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(txns) != 1 {
		t.Fatalf("want 1 txn, got %d", len(txns))
	}
}

func TestPay_NegativeAmount(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, _ := seedBasicData(t, db)

	_, err := s.Pay(ctx, svc.PayRequest{CardNum: cardNum, Amt: decimal.RequireFromString("-10.00")})
	if !errors.Is(err, svc.ErrAmountNegativeOrZero) {
		t.Fatalf("want ErrAmountNegativeOrZero, got %v", err)
	}
}

func TestPay_ExceedsBalance(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, _ := seedBasicData(t, db)

	// Balance is 500.00; try to pay 600.00.
	_, err := s.Pay(ctx, svc.PayRequest{CardNum: cardNum, Amt: decimal.RequireFromString("600.00")})
	if !errors.Is(err, svc.ErrPaymentExceedsBalance) {
		t.Fatalf("want ErrPaymentExceedsBalance, got %v", err)
	}
}

func TestPay_ZeroBalance(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, acctID := seedBasicData(t, db)

	// Set balance to zero.
	acctStore := sqlite.NewAccountStore(db)
	acct, _ := acctStore.Get(ctx, acctID)
	acct.AcctCurrBal = decimal.Zero
	if err := acctStore.Update(ctx, acct); err != nil {
		t.Fatalf("update: %v", err)
	}

	_, err := s.Pay(ctx, svc.PayRequest{CardNum: cardNum, Amt: decimal.RequireFromString("10.00")})
	if !errors.Is(err, svc.ErrZeroBalance) {
		t.Fatalf("want ErrZeroBalance, got %v", err)
	}
}

func TestPay_DecimalPrecision(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	s := newService(t, db)
	cardNum, acctID := seedBasicData(t, db)

	// Pay exactly 0.01 — tests no float drift.
	if _, err := s.Pay(ctx, svc.PayRequest{CardNum: cardNum, Amt: decimal.RequireFromString("0.01")}); err != nil {
		t.Fatalf("pay: %v", err)
	}

	acct, err := sqlite.NewAccountStore(db).Get(ctx, acctID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if !acct.AcctCurrBal.Equal(decimal.RequireFromString("499.99")) {
		t.Fatalf("balance: want 499.99, got %s", acct.AcctCurrBal)
	}
}

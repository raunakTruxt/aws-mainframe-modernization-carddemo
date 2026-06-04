package intcalc

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

func openTestDB(t *testing.T) interface{ Close() error } {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRun_basicInterestCalculation(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	// Seed: 1 account, 1 discount-group rate, 1 tran-cat-bal row.
	acctStore := sqlite.NewAccountStore(db)
	discStore := sqlite.NewDiscGroupStore(db)
	tcatStore := sqlite.NewTranCatBalStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)

	acct := &domain.AccountRecord{
		AcctID:              12345678901,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.NewFromFloat(1000.00),
		AcctCreditLimit:     decimal.NewFromFloat(5000.00),
		AcctCashCreditLimit: decimal.NewFromFloat(1000.00),
		AcctGroupID:         "GROUP001  ",
		AcctCurrCycCredit:   decimal.NewFromFloat(200.00),
		AcctCurrCycDebit:    decimal.NewFromFloat(50.00),
	}
	if err := acctStore.Create(ctx, acct); err != nil {
		t.Fatalf("create account: %v", err)
	}

	disc := &domain.DiscGroupRecord{
		DisAcctGroupID: "GROUP001  ",
		DisTranTypeCD:  "PR",
		DisTranCatCode: 1,
		DisIntRate:     decimal.NewFromFloat(24.00), // 24% APR
	}
	if err := discStore.Create(ctx, disc); err != nil {
		t.Fatalf("create discgroup: %v", err)
	}

	// catBal = 1200 => monthlyInt = (1200 * 24) / 1200 = 24.00
	tcatbal := &domain.TranCatBalRecord{
		TrancatAcctID:  12345678901,
		TrancatTypeCD:  "PR",
		TrancatCode:    1,
		TranCatBalance: decimal.NewFromFloat(1200.00),
	}
	if err := tcatStore.Create(ctx, tcatbal); err != nil {
		t.Fatalf("create tcatbal: %v", err)
	}

	// Seed card-xref so intcalc can find the card number.
	xref := &domain.CardXrefRecord{
		XrefCardNum: "4111111111111111",
		XrefCustID:  1,
		XrefAcctID:  12345678901,
	}
	if err := xrefStore.Create(ctx, xref); err != nil {
		t.Fatalf("create xref: %v", err)
	}

	var out bytes.Buffer
	fixedNow := time.Date(2022, 7, 18, 0, 0, 0, 0, time.UTC)
	s, err := Run(ctx, Config{DB: db, ParmDate: "2022071800", Now: fixedNow, Out: &out})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if s.AccountsProcessed != 1 {
		t.Errorf("accounts processed = %d, want 1", s.AccountsProcessed)
	}
	if s.TransactionsWritten != 1 {
		t.Errorf("transactions written = %d, want 1", s.TransactionsWritten)
	}

	// Verify account balance updated: 1000.00 + 24.00 = 1024.00
	updated, err := acctStore.Get(ctx, 12345678901)
	if err != nil {
		t.Fatalf("get updated account: %v", err)
	}
	wantBal := decimal.NewFromFloat(1024.00)
	if !updated.AcctCurrBal.Equal(wantBal) {
		t.Errorf("updated balance = %s, want %s", updated.AcctCurrBal, wantBal)
	}
	// Verify cycle counters reset.
	if !updated.AcctCurrCycCredit.IsZero() {
		t.Errorf("AcctCurrCycCredit not zeroed: %s", updated.AcctCurrCycCredit)
	}
	if !updated.AcctCurrCycDebit.IsZero() {
		t.Errorf("AcctCurrCycDebit not zeroed: %s", updated.AcctCurrCycDebit)
	}

	// Verify a transaction was inserted.
	tranStore := sqlite.NewTransactionStore(db)
	trans, err := tranStore.Browse(ctx, "", 0)
	if err != nil {
		t.Fatalf("browse transactions: %v", err)
	}
	if len(trans) != 1 {
		t.Fatalf("got %d transactions, want 1", len(trans))
	}
	wantAmt := decimal.NewFromFloat(24.00)
	if !trans[0].TranAmt.Equal(wantAmt) {
		t.Errorf("tran amount = %s, want %s", trans[0].TranAmt, wantAmt)
	}
}

func TestRun_zeroRateSkipped(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	acctStore := sqlite.NewAccountStore(db)
	discStore := sqlite.NewDiscGroupStore(db)
	tcatStore := sqlite.NewTranCatBalStore(db)

	acct := &domain.AccountRecord{
		AcctID:              11111111111,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.NewFromFloat(500.00),
		AcctCreditLimit:     decimal.NewFromFloat(2000.00),
		AcctCashCreditLimit: decimal.NewFromFloat(500.00),
		AcctGroupID:         "ZERO      ",
	}
	if err := acctStore.Create(ctx, acct); err != nil {
		t.Fatalf("create account: %v", err)
	}
	// Rate = 0 → no interest transaction, no account update.
	disc := &domain.DiscGroupRecord{
		DisAcctGroupID: "ZERO      ",
		DisTranTypeCD:  "PR",
		DisTranCatCode: 1,
		DisIntRate:     decimal.Zero,
	}
	if err := discStore.Create(ctx, disc); err != nil {
		t.Fatalf("create discgroup: %v", err)
	}
	tcatbal := &domain.TranCatBalRecord{
		TrancatAcctID:  11111111111,
		TrancatTypeCD:  "PR",
		TrancatCode:    1,
		TranCatBalance: decimal.NewFromFloat(1000.00),
	}
	if err := tcatStore.Create(ctx, tcatbal); err != nil {
		t.Fatalf("create tcatbal: %v", err)
	}

	s, err := Run(ctx, Config{DB: db, Out: nil})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.AccountsProcessed != 0 {
		t.Errorf("accounts processed = %d, want 0", s.AccountsProcessed)
	}
	if s.TransactionsWritten != 0 {
		t.Errorf("transactions written = %d, want 0", s.TransactionsWritten)
	}
}

func TestRun_noTcatbalRows(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	s, err := Run(ctx, Config{DB: db})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.AccountsProcessed != 0 || s.TransactionsWritten != 0 {
		t.Errorf("expected zero summary, got %+v", s)
	}
}

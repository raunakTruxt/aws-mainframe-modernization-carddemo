package tranvalidate

import (
	"bytes"
	"context"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

func TestRun_validatesAndPosts(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	acctStore := sqlite.NewAccountStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)
	custStore := sqlite.NewCustomerStore(db)
	cardStore := sqlite.NewCardStore(db)
	tranStore := sqlite.NewTransactionStore(db)

	// Seed minimal valid data.
	if err := acctStore.Create(ctx, &domain.AccountRecord{
		AcctID:              10000000001,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.NewFromFloat(500),
		AcctCreditLimit:     decimal.NewFromFloat(5000),
		AcctCashCreditLimit: decimal.NewFromFloat(1000),
		AcctGroupID:         "GRP1      ",
	}); err != nil {
		t.Fatalf("create acct: %v", err)
	}
	if err := xrefStore.Create(ctx, &domain.CardXrefRecord{
		XrefCardNum: "4111111111111111",
		XrefCustID:  1,
		XrefAcctID:  10000000001,
	}); err != nil {
		t.Fatalf("create xref: %v", err)
	}
	if err := custStore.Create(ctx, &domain.CustomerRecord{
		CustID:    1,
		CustAddrZip: "12345",
	}); err != nil {
		t.Fatalf("create cust: %v", err)
	}
	if err := cardStore.Create(ctx, &domain.CardRecord{
		CardNum:            "4111111111111111",
		CardAcctID:         10000000001,
		CardActiveStatus:   "Y",
		CardExpirationDate: "2030-12-31",
	}); err != nil {
		t.Fatalf("create card: %v", err)
	}

	// Build a 350-byte input.
	tr := &domain.TransactionRecord{
		TranID:       "TRAN0001        ",
		TranTypeCode: "PR",
		TranCatCode:  1,
		TranAmt:      decimal.NewFromFloat(100),
		TranCardNum:  "4111111111111111",
		TranOrigTS:   "2022-07-18 00:00:00.000000",
		TranProcTS:   "2022-07-18 00:00:00.000000",
	}
	b, err := tr.Encode()
	if err != nil {
		t.Fatalf("encode tran: %v", err)
	}

	s, err := Run(ctx, Config{
		Transactions: tranStore,
		CardXrefs:    xrefStore,
		Accounts:     acctStore,
		Customers:    custStore,
		Cards:        cardStore,
		Input:        bytes.NewReader(b),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Validated != 1 {
		t.Errorf("validated = %d, want 1", s.Validated)
	}
	if s.Skipped != 0 {
		t.Errorf("skipped = %d, want 0", s.Skipped)
	}

	all, _ := tranStore.Browse(ctx, "", 0)
	if len(all) != 1 {
		t.Errorf("transactions = %d, want 1", len(all))
	}
}

func TestRun_skipsUnknownCard(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	tranStore := sqlite.NewTransactionStore(db)

	tr := &domain.TransactionRecord{
		TranID:       "TRAN0002        ",
		TranTypeCode: "PR",
		TranAmt:      decimal.NewFromFloat(50),
		TranCardNum:  "9999999999999999",
		TranOrigTS:   "2022-07-18 00:00:00.000000",
		TranProcTS:   "2022-07-18 00:00:00.000000",
	}
	b, _ := tr.Encode()

	s, err := Run(ctx, Config{
		Transactions: tranStore,
		CardXrefs:    sqlite.NewCardXrefStore(db),
		Accounts:     sqlite.NewAccountStore(db),
		Customers:    sqlite.NewCustomerStore(db),
		Cards:        sqlite.NewCardStore(db),
		Input:        bytes.NewReader(b),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", s.Skipped)
	}
	if s.Validated != 0 {
		t.Errorf("validated = %d, want 0", s.Validated)
	}
}

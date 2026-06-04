package export

import (
	"bytes"
	"context"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

func TestRun_exportRoundtrip(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	custStore := sqlite.NewCustomerStore(db)
	acctStore := sqlite.NewAccountStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)
	tranStore := sqlite.NewTransactionStore(db)
	cardStore := sqlite.NewCardStore(db)

	if err := custStore.Create(ctx, &domain.CustomerRecord{
		CustID:               1,
		CustFirstName:        "JOHN",
		CustLastName:         "DOE",
		CustFICOCreditScore:  720,
		CustAddrZip:          "12345",
		CustPriCardHolderInd: "Y",
	}); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := acctStore.Create(ctx, &domain.AccountRecord{
		AcctID:              10000000001,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.NewFromFloat(1000),
		AcctCreditLimit:     decimal.NewFromFloat(5000),
		AcctCashCreditLimit: decimal.NewFromFloat(1000),
		AcctGroupID:         "GRP1      ",
	}); err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := xrefStore.Create(ctx, &domain.CardXrefRecord{
		XrefCardNum: "4111111111111111",
		XrefCustID:  1,
		XrefAcctID:  10000000001,
	}); err != nil {
		t.Fatalf("create xref: %v", err)
	}
	if err := tranStore.Create(ctx, &domain.TransactionRecord{
		TranID:      "TRAN0001        ",
		TranTypeCode: "PR",
		TranAmt:     decimal.NewFromFloat(100),
		TranCardNum: "4111111111111111",
		TranOrigTS:  "2022-07-18 00:00:00.000000",
		TranProcTS:  "2022-07-18 00:00:00.000000",
	}); err != nil {
		t.Fatalf("create tran: %v", err)
	}
	if err := cardStore.Create(ctx, &domain.CardRecord{
		CardNum:            "4111111111111111",
		CardAcctID:         10000000001,
		CardCVVCode:        123,
		CardEmbossedName:   "JOHN DOE",
		CardExpirationDate: "2030-12-31",
		CardActiveStatus:   "Y",
	}); err != nil {
		t.Fatalf("create card: %v", err)
	}

	var out bytes.Buffer
	s, err := Run(ctx, Config{
		Customers:    custStore,
		Accounts:     acctStore,
		CardXrefs:    xrefStore,
		Transactions: tranStore,
		Cards:        cardStore,
		Out:          &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if s.Customers != 1 || s.Accounts != 1 || s.CardXrefs != 1 || s.Transactions != 1 || s.Cards != 1 {
		t.Errorf("unexpected summary: %+v", s)
	}
	if s.Total() != 5 {
		t.Errorf("total = %d, want 5", s.Total())
	}

	// Output should be exactly 5 * 500 bytes.
	if out.Len() != 5*domain.ExportRecordLen {
		t.Errorf("output size = %d, want %d", out.Len(), 5*domain.ExportRecordLen)
	}

	// Verify first record type is "C" (customer).
	firstRec, err := domain.DecodeExportRecord(out.Bytes()[:domain.ExportRecordLen])
	if err != nil {
		t.Fatalf("decode first export record: %v", err)
	}
	if firstRec.RecType != "C" {
		t.Errorf("first rec type = %q, want \"C\"", firstRec.RecType)
	}
}

func TestRun_empty(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	var out bytes.Buffer
	s, err := Run(ctx, Config{
		Customers:    sqlite.NewCustomerStore(db),
		Accounts:     sqlite.NewAccountStore(db),
		CardXrefs:    sqlite.NewCardXrefStore(db),
		Transactions: sqlite.NewTransactionStore(db),
		Cards:        sqlite.NewCardStore(db),
		Out:          &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Total() != 0 {
		t.Errorf("total = %d, want 0", s.Total())
	}
	if out.Len() != 0 {
		t.Errorf("output size = %d, want 0", out.Len())
	}
}

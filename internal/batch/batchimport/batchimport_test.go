package batchimport

import (
	"bytes"
	"context"
	"testing"
	"time"

	batchexport "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/batch/export"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

func TestRun_importRoundtrip(t *testing.T) {
	srcDB, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open src db: %v", err)
	}
	defer srcDB.Close()
	dstDB, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open dst db: %v", err)
	}
	defer dstDB.Close()
	ctx := context.Background()

	// Seed source DB.
	custStore := sqlite.NewCustomerStore(srcDB)
	acctStore := sqlite.NewAccountStore(srcDB)

	if err := custStore.Create(ctx, &domain.CustomerRecord{
		CustID:               1,
		CustFirstName:        "ALICE",
		CustLastName:         "SMITH",
		CustFICOCreditScore:  700,
		CustPriCardHolderInd: "Y",
		CustAddrZip:          "10001",
	}); err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := acctStore.Create(ctx, &domain.AccountRecord{
		AcctID:              20000000001,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.NewFromFloat(200),
		AcctCreditLimit:     decimal.NewFromFloat(3000),
		AcctCashCreditLimit: decimal.NewFromFloat(500),
		AcctGroupID:         "GRP2      ",
	}); err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Export from source.
	var exported bytes.Buffer
	fixedNow := time.Date(2022, 7, 18, 0, 0, 0, 0, time.UTC)
	_, err = batchexport.Run(ctx, batchexport.Config{
		Customers:    sqlite.NewCustomerStore(srcDB),
		Accounts:     sqlite.NewAccountStore(srcDB),
		CardXrefs:    sqlite.NewCardXrefStore(srcDB),
		Transactions: sqlite.NewTransactionStore(srcDB),
		Cards:        sqlite.NewCardStore(srcDB),
		Out:          &exported,
		Now:          fixedNow,
	})
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	// Import into destination.
	s, err := Run(ctx, Config{
		Input:        &exported,
		Customers:    sqlite.NewCustomerStore(dstDB),
		Accounts:     sqlite.NewAccountStore(dstDB),
		CardXrefs:    sqlite.NewCardXrefStore(dstDB),
		Transactions: sqlite.NewTransactionStore(dstDB),
		Cards:        sqlite.NewCardStore(dstDB),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Customers != 1 {
		t.Errorf("imported customers = %d, want 1", s.Customers)
	}
	if s.Accounts != 1 {
		t.Errorf("imported accounts = %d, want 1", s.Accounts)
	}
	if s.Errors != 0 {
		t.Errorf("errors = %d, want 0", s.Errors)
	}

	// Verify customer made it into destination.
	imported, err := sqlite.NewCustomerStore(dstDB).Get(ctx, 1)
	if err != nil {
		t.Fatalf("get imported customer: %v", err)
	}
	if imported.CustFirstName != "ALICE" {
		t.Errorf("imported customer name = %q, want ALICE", imported.CustFirstName)
	}
}

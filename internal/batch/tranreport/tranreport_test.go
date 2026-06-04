package tranreport

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

func padRight16(s string) string {
	if len(s) >= 16 {
		return s[:16]
	}
	b := make([]byte, 16)
	copy(b, s)
	for i := len(s); i < 16; i++ {
		b[i] = ' '
	}
	return string(b)
}

func TestRun_basicReport(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	tranStore := sqlite.NewTransactionStore(db)
	typeStore := sqlite.NewTranTypeStore(db)
	catStore := sqlite.NewTranCatStore(db)

	if err := typeStore.Create(ctx, &domain.TranTypeRecord{TranType: "PR", TranTypeDesc: "PURCHASE"}); err != nil {
		t.Fatalf("create tran type: %v", err)
	}
	if err := catStore.Create(ctx, &domain.TranCatRecord{TranTypeCode: "PR", TranCatCode: 1, TranCatTypeDesc: "MERCHANDISE"}); err != nil {
		t.Fatalf("create tran cat: %v", err)
	}

	for i, amt := range []float64{100.00, 200.00} {
		tran := &domain.TransactionRecord{
			TranID:       padRight16(fmt.Sprintf("TRAN%012d", i+1)),
			TranTypeCode: "PR",
			TranCatCode:  1,
			TranAmt:      decimal.NewFromFloat(amt),
			TranCardNum:  "4111111111111111",
			TranOrigTS:   "2022-07-18 00:00:00.000000",
			TranSource:   "Online",
			TranDesc:     "Test",
		}
		if err := tranStore.Create(ctx, tran); err != nil {
			t.Fatalf("create tran %d: %v", i+1, err)
		}
	}

	var out bytes.Buffer
	s, err := Run(ctx, Config{
		Transactions: tranStore,
		TranTypes:    typeStore,
		TranCats:     catStore,
		CardXrefs:    sqlite.NewCardXrefStore(db),
		Out:          &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Transactions != 2 {
		t.Errorf("transactions = %d, want 2", s.Transactions)
	}
	if s.Pages < 1 {
		t.Errorf("pages = %d, want >= 1", s.Pages)
	}

	report := out.String()
	if !strings.Contains(report, "300.00") {
		t.Errorf("grand total 300.00 not found in report:\n%s", report)
	}
	if !strings.Contains(report, "CARDDEMO TRANSACTION REPORT") {
		t.Errorf("header missing in report")
	}
}

func TestRun_dateFilter(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	tranStore := sqlite.NewTransactionStore(db)

	dates := []string{"2022-07-01", "2022-07-15", "2022-07-31"}
	for i, date := range dates {
		tran := &domain.TransactionRecord{
			TranID:       padRight16(fmt.Sprintf("T%015d", i+1)),
			TranTypeCode: "PR",
			TranAmt:      decimal.NewFromFloat(10.00),
			TranCardNum:  "4111111111111111",
			TranOrigTS:   date + " 00:00:00.000000",
		}
		if err := tranStore.Create(ctx, tran); err != nil {
			t.Fatalf("create tran: %v", err)
		}
	}

	var out bytes.Buffer
	s, err := Run(ctx, Config{
		Transactions: tranStore,
		TranTypes:    sqlite.NewTranTypeStore(db),
		TranCats:     sqlite.NewTranCatStore(db),
		CardXrefs:    sqlite.NewCardXrefStore(db),
		DateFrom:     "2022-07-10",
		DateTo:       "2022-07-20",
		Out:          &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Transactions != 1 {
		t.Errorf("transactions = %d, want 1 (only 2022-07-15 in range)", s.Transactions)
	}
}

func TestRun_emptyTransactions(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	s, err := Run(ctx, Config{
		Transactions: sqlite.NewTransactionStore(db),
		TranTypes:    sqlite.NewTranTypeStore(db),
		TranCats:     sqlite.NewTranCatStore(db),
		CardXrefs:    sqlite.NewCardXrefStore(db),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Transactions != 0 {
		t.Errorf("transactions = %d, want 0", s.Transactions)
	}
}

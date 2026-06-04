package acctdump

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

func TestRun_basic(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	acctStore := sqlite.NewAccountStore(db)
	for i := 1; i <= 3; i++ {
		if err := acctStore.Create(ctx, &domain.AccountRecord{
			AcctID:              int64(i),
			AcctActiveStatus:    "Y",
			AcctCurrBal:         decimal.NewFromFloat(float64(i) * 100),
			AcctCreditLimit:     decimal.NewFromFloat(5000),
			AcctCashCreditLimit: decimal.NewFromFloat(1000),
			AcctGroupID:         "GRP1      ",
		}); err != nil {
			t.Fatalf("create account %d: %v", i, err)
		}
	}

	var out bytes.Buffer
	s, err := Run(ctx, Config{Accounts: acctStore, Out: &out})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Count != 3 {
		t.Errorf("count = %d, want 3", s.Count)
	}
	if !strings.Contains(out.String(), "Total accounts: 3") {
		t.Errorf("summary not found in output: %s", out.String())
	}
}

func TestRun_empty(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	s, err := Run(context.Background(), Config{Accounts: sqlite.NewAccountStore(db)})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.Count != 0 {
		t.Errorf("count = %d, want 0", s.Count)
	}
}

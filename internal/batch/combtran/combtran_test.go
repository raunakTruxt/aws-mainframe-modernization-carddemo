package combtran

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

func TestRun_mergeNoSecondary(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	tranStore := sqlite.NewTransactionStore(db)
	for i := 1; i <= 3; i++ {
		if err := tranStore.Create(ctx, &domain.TransactionRecord{
			TranID:       padRight16(fmt.Sprintf("T%015d", i)),
			TranTypeCode: "PR",
			TranAmt:      decimal.NewFromFloat(10),
			TranCardNum:  "4111111111111111",
			TranOrigTS:   "2022-07-18 00:00:00.000000",
			TranProcTS:   "2022-07-18 00:00:00.000000",
		}); err != nil {
			t.Fatalf("create tran: %v", err)
		}
	}

	s, err := Run(ctx, Config{
		Primary: tranStore,
		Target:  tranStore,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.FromPrimary != 3 {
		t.Errorf("fromPrimary = %d, want 3", s.FromPrimary)
	}
	if s.FromSecondary != 0 {
		t.Errorf("fromSecondary = %d, want 0", s.FromSecondary)
	}
}

func TestRun_mergeWithSecondary(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	targetDB, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer targetDB.Close()

	primaryStore := sqlite.NewTransactionStore(db)
	targetStore := sqlite.NewTransactionStore(targetDB)

	// Primary has 2 transactions.
	for i := 1; i <= 2; i++ {
		if err := primaryStore.Create(ctx, &domain.TransactionRecord{
			TranID:       padRight16(fmt.Sprintf("T%015d", i)),
			TranTypeCode: "PR",
			TranAmt:      decimal.NewFromFloat(10),
			TranCardNum:  "4111111111111111",
			TranOrigTS:   "2022-07-18 00:00:00.000000",
			TranProcTS:   "2022-07-18 00:00:00.000000",
		}); err != nil {
			t.Fatalf("create primary tran: %v", err)
		}
	}

	// Secondary has 1 new + 1 duplicate of primary.
	secondaryTrans := []*domain.TransactionRecord{
		{
			TranID:       padRight16("T000000000000002"), // duplicate
			TranTypeCode: "PR",
			TranAmt:      decimal.NewFromFloat(10),
			TranCardNum:  "4111111111111111",
			TranOrigTS:   "2022-07-18 00:00:00.000000",
			TranProcTS:   "2022-07-18 00:00:00.000000",
		},
		{
			TranID:       padRight16("T000000000000003"), // new
			TranTypeCode: "PR",
			TranAmt:      decimal.NewFromFloat(20),
			TranCardNum:  "4111111111111111",
			TranOrigTS:   "2022-07-18 00:00:00.000000",
			TranProcTS:   "2022-07-18 00:00:00.000000",
		},
	}
	var secBuf bytes.Buffer
	for _, tr := range secondaryTrans {
		b, _ := tr.Encode()
		secBuf.Write(b)
	}

	s, err := Run(ctx, Config{
		Primary:   primaryStore,
		Secondary: &secBuf,
		Target:    targetStore,
		Out:       nil,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.FromPrimary != 2 {
		t.Errorf("fromPrimary = %d, want 2", s.FromPrimary)
	}
	if s.FromSecondary != 1 {
		t.Errorf("fromSecondary = %d, want 1 (duplicate deduped)", s.FromSecondary)
	}
	if s.Duplicates != 1 {
		t.Errorf("duplicates = %d, want 1", s.Duplicates)
	}
	if s.Written != 3 {
		t.Errorf("written = %d, want 3", s.Written)
	}

	all, _ := targetStore.Browse(ctx, "", 0)
	if len(all) != 3 {
		t.Errorf("target records = %d, want 3", len(all))
	}
}

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

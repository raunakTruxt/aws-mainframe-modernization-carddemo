package tranbkp

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	"github.com/shopspring/decimal"
)

func TestRun_backupAndReset(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	tranStore := sqlite.NewTransactionStore(db)
	for i := 1; i <= 3; i++ {
		tran := &domain.TransactionRecord{
			TranID:       padRight(fmt.Sprintf("T%015d", i), 16),
			TranTypeCode: "PR",
			TranAmt:      decimal.NewFromFloat(float64(i) * 10),
			TranCardNum:  "4111111111111111",
			TranOrigTS:   "2022-07-18 00:00:00.000000",
			TranProcTS:   "2022-07-18 00:00:00.000000",
		}
		if err := tranStore.Create(ctx, tran); err != nil {
			t.Fatalf("create tran %d: %v", i, err)
		}
	}

	var backup bytes.Buffer
	s, err := Run(ctx, Config{
		Transactions: tranStore,
		Backup:       &backup,
		Reset:        true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.BackedUp != 3 {
		t.Errorf("backed up = %d, want 3", s.BackedUp)
	}
	if s.Deleted != 3 {
		t.Errorf("deleted = %d, want 3", s.Deleted)
	}

	// Backup should be 3 * 350 bytes.
	if backup.Len() != 3*domain.TransactionRecordLen {
		t.Errorf("backup size = %d, want %d", backup.Len(), 3*domain.TransactionRecordLen)
	}

	// DB should be empty after reset.
	all, _ := tranStore.Browse(ctx, "", 0)
	if len(all) != 0 {
		t.Errorf("transactions remaining = %d, want 0", len(all))
	}

	// Restore from backup.
	restored, err := Restore(ctx, &backup, tranStore, nil)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored != 3 {
		t.Errorf("restored = %d, want 3", restored)
	}
	all, _ = tranStore.Browse(ctx, "", 0)
	if len(all) != 3 {
		t.Errorf("transactions after restore = %d, want 3", len(all))
	}
}

func TestRun_backupWithoutReset(t *testing.T) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ctx := context.Background()

	tranStore := sqlite.NewTransactionStore(db)
	if err := tranStore.Create(ctx, &domain.TransactionRecord{
		TranID:      "T001            ",
		TranTypeCode: "PR",
		TranAmt:     decimal.NewFromFloat(50),
		TranCardNum: "4111111111111111",
		TranOrigTS:  "2022-07-18 00:00:00.000000",
		TranProcTS:  "2022-07-18 00:00:00.000000",
	}); err != nil {
		t.Fatalf("create tran: %v", err)
	}

	var backup bytes.Buffer
	s, err := Run(ctx, Config{Transactions: tranStore, Backup: &backup, Reset: false})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if s.BackedUp != 1 {
		t.Errorf("backed up = %d, want 1", s.BackedUp)
	}
	if s.Deleted != 0 {
		t.Errorf("deleted = %d, want 0", s.Deleted)
	}
	// DB still has the record.
	all, _ := tranStore.Browse(ctx, "", 0)
	if len(all) != 1 {
		t.Errorf("transactions = %d, want 1", len(all))
	}
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	b := make([]byte, n)
	copy(b, s)
	for i := len(s); i < n; i++ {
		b[i] = ' '
	}
	return string(b)
}

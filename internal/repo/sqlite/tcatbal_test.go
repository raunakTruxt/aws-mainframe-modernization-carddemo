package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/shopspring/decimal"
)

func sampleTCatBal(acctID int64, typeCD string, catCD int64, bal string) *domain.TranCatBalRecord {
	return &domain.TranCatBalRecord{
		TrancatAcctID:  acctID,
		TrancatTypeCD:  typeCD,
		TrancatCode:    catCD,
		TranCatBalance: decimal.RequireFromString(bal),
	}
}

func TestTranCatBalStore(t *testing.T) {
	ctx := context.Background()
	s := NewTranCatBalStore(newTestDB(t))

	tb1 := sampleTCatBal(100, "01", 1000, "50.00")
	tb2 := sampleTCatBal(100, "02", 1000, "75.00")
	tb3 := sampleTCatBal(200, "01", 1000, "10.00")

	for _, tb := range []*domain.TranCatBalRecord{tb1, tb2, tb3} {
		if err := s.Create(ctx, tb); err != nil {
			t.Fatalf("create (%d,%s,%d): %v", tb.TrancatAcctID, tb.TrancatTypeCD, tb.TrancatCode, err)
		}
	}

	// Full-field Get assertion.
	got, err := s.Get(ctx, 100, "01", 1000)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TrancatAcctID != tb1.TrancatAcctID || got.TrancatTypeCD != tb1.TrancatTypeCD ||
		got.TrancatCode != tb1.TrancatCode || !got.TranCatBalance.Equal(tb1.TranCatBalance) {
		t.Fatalf("get mismatch:\nwant %+v\ngot  %+v", tb1, got)
	}

	// Negative balance round-trip.
	neg := sampleTCatBal(300, "01", 1000, "-25.00")
	if err := s.Create(ctx, neg); err != nil {
		t.Fatalf("create negative: %v", err)
	}
	gotNeg, _ := s.Get(ctx, 300, "01", 1000)
	if !gotNeg.TranCatBalance.Equal(decimal.RequireFromString("-25.00")) {
		t.Fatalf("negative balance mismatch: %v", gotNeg.TranCatBalance)
	}

	// GetByAccount: count and content — each result must have TrancatAcctID=100.
	byAcct, err := s.GetByAccount(ctx, 100)
	if err != nil {
		t.Fatalf("get-by-account: %v", err)
	}
	if len(byAcct) != 2 {
		t.Fatalf("get-by-account: want 2, got %d", len(byAcct))
	}
	for _, tb := range byAcct {
		if tb.TrancatAcctID != 100 {
			t.Fatalf("get-by-account: wrong acct %d in result %+v", tb.TrancatAcctID, tb)
		}
	}

	// Update and verify.
	got.TranCatBalance = decimal.RequireFromString("60.00")
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := s.Get(ctx, 100, "01", 1000)
	if !reread.TranCatBalance.Equal(decimal.RequireFromString("60.00")) {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update of missing row returns ErrNotFound.
	if err := s.Update(ctx, sampleTCatBal(999, "99", 9999, "0")); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}

	// Upsert: insert a new row, then update it in place.
	if err := s.Upsert(ctx, sampleTCatBal(400, "01", 1000, "5.00")); err != nil {
		t.Fatalf("upsert insert: %v", err)
	}
	if err := s.Upsert(ctx, sampleTCatBal(400, "01", 1000, "7.50")); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	upserted, _ := s.Get(ctx, 400, "01", 1000)
	if !upserted.TranCatBalance.Equal(decimal.RequireFromString("7.50")) {
		t.Fatalf("upsert not persisted: %+v", upserted)
	}

	// List — 5 rows: tb1, tb2, tb3, neg(300), upserted(400).
	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("list: want 5, got %d", len(all))
	}

	// Delete.
	if err := s.Delete(ctx, 200, "01", 1000); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, 200, "01", 1000); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}

	// Delete of missing row returns ErrNotFound.
	if err := s.Delete(ctx, 200, "01", 1000); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}

	// Duplicate Create returns ErrConflict.
	if err := s.Create(ctx, tb1); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}
}

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

	if err := s.Create(ctx, sampleTCatBal(100, "01", 1000, "50.00")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, sampleTCatBal(100, "02", 1000, "75.00")); err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if err := s.Create(ctx, sampleTCatBal(200, "01", 1000, "10.00")); err != nil {
		t.Fatalf("create 3: %v", err)
	}

	got, err := s.Get(ctx, 100, "01", 1000)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.TranCatBalance.Equal(decimal.RequireFromString("50.00")) {
		t.Fatalf("get mismatch: %+v", got)
	}

	byAcct, err := s.GetByAccount(ctx, 100)
	if err != nil {
		t.Fatalf("get-by-account: %v", err)
	}
	if len(byAcct) != 2 {
		t.Fatalf("get-by-account: want 2, got %d", len(byAcct))
	}

	got.TranCatBalance = decimal.RequireFromString("60.00")
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := s.Get(ctx, 100, "01", 1000)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if !reread.TranCatBalance.Equal(decimal.RequireFromString("60.00")) {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Upsert: insert a new row, then update it in place.
	if err := s.Upsert(ctx, sampleTCatBal(300, "01", 1000, "5.00")); err != nil {
		t.Fatalf("upsert insert: %v", err)
	}
	if err := s.Upsert(ctx, sampleTCatBal(300, "01", 1000, "7.50")); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	upserted, err := s.Get(ctx, 300, "01", 1000)
	if err != nil {
		t.Fatalf("get upserted: %v", err)
	}
	if !upserted.TranCatBalance.Equal(decimal.RequireFromString("7.50")) {
		t.Fatalf("upsert not persisted: %+v", upserted)
	}

	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("list: want 4, got %d", len(all))
	}

	if err := s.Delete(ctx, 200, "01", 1000); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, 200, "01", 1000); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}
}

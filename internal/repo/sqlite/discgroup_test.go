package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/shopspring/decimal"
)

func sampleDiscGroup(groupID, typeCD string, catCD int64, rate string) *domain.DiscGroupRecord {
	return &domain.DiscGroupRecord{
		DisAcctGroupID: groupID,
		DisTranTypeCD:  typeCD,
		DisTranCatCode: catCD,
		DisIntRate:     decimal.RequireFromString(rate),
	}
}

func TestDiscGroupStore(t *testing.T) {
	ctx := context.Background()
	s := NewDiscGroupStore(newTestDB(t))

	if err := s.Create(ctx, sampleDiscGroup("GRP1", "01", 1000, "12.50")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, sampleDiscGroup("GRP1", "02", 1000, "9.99")); err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if err := s.Create(ctx, sampleDiscGroup("GRP2", "01", 1000, "15.00")); err != nil {
		t.Fatalf("create 3: %v", err)
	}

	got, err := s.Get(ctx, "GRP1", "01", 1000)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.DisIntRate.Equal(decimal.RequireFromString("12.50")) {
		t.Fatalf("get mismatch: %+v", got)
	}

	byGroup, err := s.GetByGroup(ctx, "GRP1")
	if err != nil {
		t.Fatalf("get-by-group: %v", err)
	}
	if len(byGroup) != 2 {
		t.Fatalf("get-by-group: want 2, got %d", len(byGroup))
	}

	got.DisIntRate = decimal.RequireFromString("20.00")
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := s.Get(ctx, "GRP1", "01", 1000)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if !reread.DisIntRate.Equal(decimal.RequireFromString("20.00")) {
		t.Fatalf("update not persisted: %+v", reread)
	}

	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("list: want 3, got %d", len(all))
	}

	if err := s.Delete(ctx, "GRP2", "01", 1000); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "GRP2", "01", 1000); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}
}

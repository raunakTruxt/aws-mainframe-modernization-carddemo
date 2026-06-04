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

	dg1 := sampleDiscGroup("GRP1", "01", 1000, "12.50")
	dg2 := sampleDiscGroup("GRP1", "02", 1000, "9.99")
	dg3 := sampleDiscGroup("GRP2", "01", 1000, "15.00")

	for _, dg := range []*domain.DiscGroupRecord{dg1, dg2, dg3} {
		if err := s.Create(ctx, dg); err != nil {
			t.Fatalf("create (%s,%s,%d): %v", dg.DisAcctGroupID, dg.DisTranTypeCD, dg.DisTranCatCode, err)
		}
	}

	// Full-field Get assertion.
	got, err := s.Get(ctx, "GRP1", "01", 1000)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.DisAcctGroupID != dg1.DisAcctGroupID || got.DisTranTypeCD != dg1.DisTranTypeCD ||
		got.DisTranCatCode != dg1.DisTranCatCode || !got.DisIntRate.Equal(dg1.DisIntRate) {
		t.Fatalf("get mismatch:\nwant %+v\ngot  %+v", dg1, got)
	}

	// Negative rate round-trip.
	neg := sampleDiscGroup("GRP3", "01", 1000, "-5.00")
	if err := s.Create(ctx, neg); err != nil {
		t.Fatalf("create negative: %v", err)
	}
	gotNeg, _ := s.Get(ctx, "GRP3", "01", 1000)
	if !gotNeg.DisIntRate.Equal(decimal.RequireFromString("-5.00")) {
		t.Fatalf("negative rate mismatch: %v", gotNeg.DisIntRate)
	}

	// GetByGroup: count and content — each result must have DisAcctGroupID="GRP1".
	byGroup, err := s.GetByGroup(ctx, "GRP1")
	if err != nil {
		t.Fatalf("get-by-group: %v", err)
	}
	if len(byGroup) != 2 {
		t.Fatalf("get-by-group: want 2, got %d", len(byGroup))
	}
	for _, dg := range byGroup {
		if dg.DisAcctGroupID != "GRP1" {
			t.Fatalf("get-by-group: wrong group %q in result %+v", dg.DisAcctGroupID, dg)
		}
	}

	// Update and verify.
	got.DisIntRate = decimal.RequireFromString("20.00")
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := s.Get(ctx, "GRP1", "01", 1000)
	if !reread.DisIntRate.Equal(decimal.RequireFromString("20.00")) {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update of missing row returns ErrNotFound.
	if err := s.Update(ctx, sampleDiscGroup("MISS", "99", 9999, "0")); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}

	// List.
	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("list: want 4 (including negative), got %d", len(all))
	}

	// Delete.
	if err := s.Delete(ctx, "GRP2", "01", 1000); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "GRP2", "01", 1000); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}

	// Delete of missing row returns ErrNotFound.
	if err := s.Delete(ctx, "GRP2", "01", 1000); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}

	// Duplicate Create returns ErrConflict.
	if err := s.Create(ctx, dg1); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}
}

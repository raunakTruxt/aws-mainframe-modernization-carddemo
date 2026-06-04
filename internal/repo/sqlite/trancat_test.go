package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

func TestTranCatStore(t *testing.T) {
	ctx := context.Background()
	s := NewTranCatStore(newTestDB(t))

	tc1 := &domain.TranCatRecord{TranTypeCode: "01", TranCatCode: 1000, TranCatTypeDesc: "RETAIL"}
	tc2 := &domain.TranCatRecord{TranTypeCode: "01", TranCatCode: 2000, TranCatTypeDesc: "ONLINE"}
	tc3 := &domain.TranCatRecord{TranTypeCode: "02", TranCatCode: 1000, TranCatTypeDesc: "CASH"}

	for _, tc := range []*domain.TranCatRecord{tc1, tc2, tc3} {
		if err := s.Create(ctx, tc); err != nil {
			t.Fatalf("create (%s,%d): %v", tc.TranTypeCode, tc.TranCatCode, err)
		}
	}

	// Full-field Get assertion.
	got, err := s.Get(ctx, "01", 1000)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TranTypeCode != tc1.TranTypeCode || got.TranCatCode != tc1.TranCatCode || got.TranCatTypeDesc != tc1.TranCatTypeDesc {
		t.Fatalf("get mismatch:\nwant %+v\ngot  %+v", tc1, got)
	}

	// GetByType: count and content — each result must have TranTypeCode="01".
	byType, err := s.GetByType(ctx, "01")
	if err != nil {
		t.Fatalf("get-by-type: %v", err)
	}
	if len(byType) != 2 {
		t.Fatalf("get-by-type: want 2, got %d", len(byType))
	}
	for _, tc := range byType {
		if tc.TranTypeCode != "01" {
			t.Fatalf("get-by-type: wrong type code %q in result %+v", tc.TranTypeCode, tc)
		}
	}

	// Update and verify.
	got.TranCatTypeDesc = "IN STORE"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := s.Get(ctx, "01", 1000)
	if reread.TranCatTypeDesc != "IN STORE" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update of missing row returns ErrNotFound.
	if err := s.Update(ctx, &domain.TranCatRecord{TranTypeCode: "99", TranCatCode: 9999, TranCatTypeDesc: "X"}); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}

	// List — 3 rows total.
	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("list: want 3, got %d", len(all))
	}

	// Delete.
	if err := s.Delete(ctx, "02", 1000); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "02", 1000); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}

	// Delete of missing row returns ErrNotFound.
	if err := s.Delete(ctx, "02", 1000); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}

	// Duplicate Create returns ErrConflict.
	if err := s.Create(ctx, tc1); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}
}

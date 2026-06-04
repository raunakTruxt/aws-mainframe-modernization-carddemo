package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

func TestTranTypeStore(t *testing.T) {
	ctx := context.Background()
	s := NewTranTypeStore(newTestDB(t))

	tt1 := &domain.TranTypeRecord{TranType: "01", TranTypeDesc: "PURCHASE"}
	tt2 := &domain.TranTypeRecord{TranType: "02", TranTypeDesc: "PAYMENT"}

	if err := s.Create(ctx, tt1); err != nil {
		t.Fatalf("create 01: %v", err)
	}
	if err := s.Create(ctx, tt2); err != nil {
		t.Fatalf("create 02: %v", err)
	}

	// Full-field Get assertion.
	got, err := s.Get(ctx, "01")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TranType != tt1.TranType || got.TranTypeDesc != tt1.TranTypeDesc {
		t.Fatalf("get mismatch:\nwant %+v\ngot  %+v", tt1, got)
	}

	// Update and verify.
	got.TranTypeDesc = "SALE"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := s.Get(ctx, "01")
	if reread.TranTypeDesc != "SALE" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update of missing row returns ErrNotFound.
	if err := s.Update(ctx, &domain.TranTypeRecord{TranType: "99", TranTypeDesc: "X"}); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}

	// List.
	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("list: want 2, got %d", len(all))
	}
	// Verify list is ordered by tran_type.
	if all[0].TranType != "01" || all[1].TranType != "02" {
		t.Fatalf("list order: %v", all)
	}

	// Delete.
	if err := s.Delete(ctx, "02"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "02"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}

	// Delete of missing row returns ErrNotFound.
	if err := s.Delete(ctx, "02"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}

	// Duplicate Create returns ErrConflict.
	if err := s.Create(ctx, tt1); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}
}

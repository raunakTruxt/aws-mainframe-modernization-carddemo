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

	if err := s.Create(ctx, &domain.TranTypeRecord{TranType: "01", TranTypeDesc: "PURCHASE"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, &domain.TranTypeRecord{TranType: "02", TranTypeDesc: "PAYMENT"}); err != nil {
		t.Fatalf("create 2: %v", err)
	}

	got, err := s.Get(ctx, "01")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TranTypeDesc != "PURCHASE" {
		t.Fatalf("get mismatch: %+v", got)
	}

	got.TranTypeDesc = "SALE"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := s.Get(ctx, "01")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if reread.TranTypeDesc != "SALE" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("list: want 2, got %d", len(all))
	}

	if err := s.Delete(ctx, "02"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "02"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}
}

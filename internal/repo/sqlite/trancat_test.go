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

	if err := s.Create(ctx, &domain.TranCatRecord{TranTypeCode: "01", TranCatCode: 1000, TranCatTypeDesc: "RETAIL"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, &domain.TranCatRecord{TranTypeCode: "01", TranCatCode: 2000, TranCatTypeDesc: "ONLINE"}); err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if err := s.Create(ctx, &domain.TranCatRecord{TranTypeCode: "02", TranCatCode: 1000, TranCatTypeDesc: "CASH"}); err != nil {
		t.Fatalf("create 3: %v", err)
	}

	got, err := s.Get(ctx, "01", 1000)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TranCatTypeDesc != "RETAIL" {
		t.Fatalf("get mismatch: %+v", got)
	}

	byType, err := s.GetByType(ctx, "01")
	if err != nil {
		t.Fatalf("get-by-type: %v", err)
	}
	if len(byType) != 2 {
		t.Fatalf("get-by-type: want 2, got %d", len(byType))
	}

	got.TranCatTypeDesc = "IN STORE"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := s.Get(ctx, "01", 1000)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if reread.TranCatTypeDesc != "IN STORE" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	all, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("list: want 3, got %d", len(all))
	}

	if err := s.Delete(ctx, "02", 1000); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "02", 1000); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}
}

package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

func sampleXref(num string, custID, acctID int64) *domain.CardXrefRecord {
	return &domain.CardXrefRecord{XrefCardNum: num, XrefCustID: custID, XrefAcctID: acctID}
}

func TestCardXrefStore(t *testing.T) {
	ctx := context.Background()
	s := NewCardXrefStore(newTestDB(t))

	if err := s.Create(ctx, sampleXref("4111111111111111", 10, 100)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, sampleXref("4222222222222222", 10, 200)); err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if err := s.Create(ctx, sampleXref("4333333333333333", 20, 100)); err != nil {
		t.Fatalf("create 3: %v", err)
	}

	got, err := s.Get(ctx, "4111111111111111")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.XrefCustID != 10 || got.XrefAcctID != 100 {
		t.Fatalf("get mismatch: %+v", got)
	}

	byAcct, err := s.GetByAccount(ctx, 100)
	if err != nil {
		t.Fatalf("get-by-account: %v", err)
	}
	if len(byAcct) != 2 {
		t.Fatalf("get-by-account: want 2, got %d", len(byAcct))
	}

	byCust, err := s.GetByCustomer(ctx, 10)
	if err != nil {
		t.Fatalf("get-by-customer: %v", err)
	}
	if len(byCust) != 2 {
		t.Fatalf("get-by-customer: want 2, got %d", len(byCust))
	}

	got.XrefAcctID = 999
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := s.Get(ctx, "4111111111111111")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if reread.XrefAcctID != 999 {
		t.Fatalf("update not persisted: %+v", reread)
	}

	all, err := s.Browse(ctx, "", 10)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("browse: want 3, got %d", len(all))
	}

	if err := s.Delete(ctx, "4333333333333333"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "4333333333333333"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}
}

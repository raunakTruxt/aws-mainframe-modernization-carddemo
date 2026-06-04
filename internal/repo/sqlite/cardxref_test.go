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

	x1 := sampleXref("4111111111111111", 10, 100)
	x2 := sampleXref("4222222222222222", 10, 200)
	x3 := sampleXref("4333333333333333", 20, 100)

	for _, x := range []*domain.CardXrefRecord{x1, x2, x3} {
		if err := s.Create(ctx, x); err != nil {
			t.Fatalf("create %s: %v", x.XrefCardNum, err)
		}
	}

	// Full-field Get assertion.
	got, err := s.Get(ctx, "4111111111111111")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.XrefCardNum != x1.XrefCardNum || got.XrefCustID != x1.XrefCustID || got.XrefAcctID != x1.XrefAcctID {
		t.Fatalf("get mismatch:\nwant %+v\ngot  %+v", x1, got)
	}

	// GetByAccount: count and content — each result must have XrefAcctID=100.
	byAcct, err := s.GetByAccount(ctx, 100)
	if err != nil {
		t.Fatalf("get-by-account: %v", err)
	}
	if len(byAcct) != 2 {
		t.Fatalf("get-by-account: want 2, got %d", len(byAcct))
	}
	for _, x := range byAcct {
		if x.XrefAcctID != 100 {
			t.Fatalf("get-by-account: wrong XrefAcctID %d in result %+v", x.XrefAcctID, x)
		}
	}

	// GetByCustomer: count and content — each result must have XrefCustID=10.
	byCust, err := s.GetByCustomer(ctx, 10)
	if err != nil {
		t.Fatalf("get-by-customer: %v", err)
	}
	if len(byCust) != 2 {
		t.Fatalf("get-by-customer: want 2, got %d", len(byCust))
	}
	for _, x := range byCust {
		if x.XrefCustID != 10 {
			t.Fatalf("get-by-customer: wrong XrefCustID %d in result %+v", x.XrefCustID, x)
		}
	}

	// Update and verify.
	got.XrefAcctID = 999
	got.XrefCustID = 77
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := s.Get(ctx, "4111111111111111")
	if reread.XrefAcctID != 999 || reread.XrefCustID != 77 {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update of missing row returns ErrNotFound.
	if err := s.Update(ctx, sampleXref("9999999999999999", 0, 0)); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}

	// Browse full range.
	all, err := s.Browse(ctx, "", 0)
	if err != nil {
		t.Fatalf("browse all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("browse all: want 3, got %d", len(all))
	}

	// Browse cursor: start from "4222..." → 2 results.
	page, err := s.Browse(ctx, "4222222222222222", 10)
	if err != nil {
		t.Fatalf("browse cursor: %v", err)
	}
	if len(page) != 2 || page[0].XrefCardNum != "4222222222222222" {
		t.Fatalf("browse cursor: unexpected %v", page)
	}

	// Delete.
	if err := s.Delete(ctx, "4333333333333333"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "4333333333333333"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}

	// Delete of missing row returns ErrNotFound.
	if err := s.Delete(ctx, "4333333333333333"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}

	// Duplicate Create returns ErrConflict.
	if err := s.Create(ctx, sampleXref("4222222222222222", 10, 200)); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}
}

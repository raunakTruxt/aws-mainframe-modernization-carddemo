package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/shopspring/decimal"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func sampleAccount(id int64) *domain.AccountRecord {
	return &domain.AccountRecord{
		AcctID:              id,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.RequireFromString("123.45"),
		AcctCreditLimit:     decimal.RequireFromString("5000.00"),
		AcctCashCreditLimit: decimal.RequireFromString("1000.00"),
		AcctOpenDate:        "2020-01-01",
		AcctExpirationDate:  "2025-01-01",
		AcctReissueDate:     "2023-01-01",
		AcctCurrCycCredit:   decimal.RequireFromString("10.00"),
		AcctCurrCycDebit:    decimal.RequireFromString("20.00"),
		AcctAddrZip:         "90210",
		AcctGroupID:         "GRP1",
	}
}

func assertAccountEqual(t *testing.T, want, got *domain.AccountRecord) {
	t.Helper()
	if got.AcctID != want.AcctID ||
		got.AcctActiveStatus != want.AcctActiveStatus ||
		!got.AcctCurrBal.Equal(want.AcctCurrBal) ||
		!got.AcctCreditLimit.Equal(want.AcctCreditLimit) ||
		!got.AcctCashCreditLimit.Equal(want.AcctCashCreditLimit) ||
		got.AcctOpenDate != want.AcctOpenDate ||
		got.AcctExpirationDate != want.AcctExpirationDate ||
		got.AcctReissueDate != want.AcctReissueDate ||
		!got.AcctCurrCycCredit.Equal(want.AcctCurrCycCredit) ||
		!got.AcctCurrCycDebit.Equal(want.AcctCurrCycDebit) ||
		got.AcctAddrZip != want.AcctAddrZip ||
		got.AcctGroupID != want.AcctGroupID {
		t.Fatalf("account mismatch:\nwant %+v\ngot  %+v", want, got)
	}
}

func TestAccountStore(t *testing.T) {
	ctx := context.Background()
	s := NewAccountStore(newTestDB(t))

	a1 := sampleAccount(1)
	a2 := sampleAccount(2)
	a3 := sampleAccount(3)

	for _, a := range []*domain.AccountRecord{a1, a2, a3} {
		if err := s.Create(ctx, a); err != nil {
			t.Fatalf("create %d: %v", a.AcctID, err)
		}
	}

	// Full-field Get assertion.
	got, err := s.Get(ctx, 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	assertAccountEqual(t, a1, got)

	// Negative decimal round-trip.
	neg := sampleAccount(4)
	neg.AcctCurrBal = decimal.RequireFromString("-99.99")
	neg.AcctCurrCycDebit = decimal.RequireFromString("-0.01")
	if err := s.Create(ctx, neg); err != nil {
		t.Fatalf("create negative: %v", err)
	}
	gotNeg, err := s.Get(ctx, 4)
	if err != nil {
		t.Fatalf("get negative: %v", err)
	}
	if !gotNeg.AcctCurrBal.Equal(neg.AcctCurrBal) || !gotNeg.AcctCurrCycDebit.Equal(neg.AcctCurrCycDebit) {
		t.Fatalf("negative decimal mismatch: %+v", gotNeg)
	}

	// Update and verify all changed fields.
	got.AcctActiveStatus = "N"
	got.AcctCurrBal = decimal.RequireFromString("999.99")
	got.AcctGroupID = "GRP2"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := s.Get(ctx, 1)
	if reread.AcctActiveStatus != "N" || !reread.AcctCurrBal.Equal(decimal.RequireFromString("999.99")) || reread.AcctGroupID != "GRP2" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update of missing row returns ErrNotFound.
	if err := s.Update(ctx, sampleAccount(999)); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}

	// Browse full range returns all 4 rows in PK order.
	all, err := s.Browse(ctx, 0, 0)
	if err != nil {
		t.Fatalf("browse all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("browse all: want 4, got %d", len(all))
	}

	// Browse cursor: start from ID 2, expect IDs 2, 3, 4 with limit 3.
	page, err := s.Browse(ctx, 2, 3)
	if err != nil {
		t.Fatalf("browse cursor: %v", err)
	}
	if len(page) != 3 || page[0].AcctID != 2 || page[1].AcctID != 3 || page[2].AcctID != 4 {
		t.Fatalf("browse cursor: want [2,3,4], got %v", idsOf(page))
	}

	// Delete.
	if err := s.Delete(ctx, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, 1); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}

	// Delete of missing row returns ErrNotFound.
	if err := s.Delete(ctx, 1); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}

	// Duplicate Create returns ErrConflict.
	if err := s.Create(ctx, sampleAccount(2)); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}
}

func idsOf(recs []*domain.AccountRecord) []int64 {
	ids := make([]int64, len(recs))
	for i, r := range recs {
		ids[i] = r.AcctID
	}
	return ids
}

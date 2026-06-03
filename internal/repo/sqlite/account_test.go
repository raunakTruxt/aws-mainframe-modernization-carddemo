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

func TestAccountStore(t *testing.T) {
	ctx := context.Background()
	s := NewAccountStore(newTestDB(t))

	if err := s.Create(ctx, sampleAccount(1)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, sampleAccount(2)); err != nil {
		t.Fatalf("create 2: %v", err)
	}

	got, err := s.Get(ctx, 1)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.AcctID != 1 || !got.AcctCurrBal.Equal(decimal.RequireFromString("123.45")) {
		t.Fatalf("get mismatch: %+v", got)
	}

	got.AcctActiveStatus = "N"
	got.AcctCurrBal = decimal.RequireFromString("999.99")
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := s.Get(ctx, 1)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if reread.AcctActiveStatus != "N" || !reread.AcctCurrBal.Equal(decimal.RequireFromString("999.99")) {
		t.Fatalf("update not persisted: %+v", reread)
	}

	all, err := s.Browse(ctx, 0, 10)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(all) != 2 || all[0].AcctID != 1 || all[1].AcctID != 2 {
		t.Fatalf("browse mismatch: %+v", all)
	}

	if err := s.Delete(ctx, 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, 1); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}

	if err := s.Create(ctx, sampleAccount(2)); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}
}

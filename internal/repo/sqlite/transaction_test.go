package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/shopspring/decimal"
)

func sampleTransaction(id, cardNum string) *domain.TransactionRecord {
	return &domain.TransactionRecord{
		TranID:           id,
		TranTypeCode:     "01",
		TranCatCode:      1000,
		TranSource:       "POS",
		TranDesc:         "PURCHASE",
		TranAmt:          decimal.RequireFromString("42.50"),
		TranMerchantID:   555,
		TranMerchantName: "ACME",
		TranMerchantCity: "CITY",
		TranMerchantZip:  "90210",
		TranCardNum:      cardNum,
		TranOrigTS:       "2024-01-01-00.00.00.000000",
		TranProcTS:       "2024-01-01-00.00.01.000000",
	}
}

func TestTransactionStore(t *testing.T) {
	ctx := context.Background()
	s := NewTransactionStore(newTestDB(t))

	t1 := sampleTransaction("T0000000000000001", "4111111111111111")
	t2 := sampleTransaction("T0000000000000002", "4111111111111111")
	t3 := sampleTransaction("T0000000000000003", "4222222222222222")

	for _, tx := range []*domain.TransactionRecord{t1, t2, t3} {
		if err := s.Create(ctx, tx); err != nil {
			t.Fatalf("create %s: %v", tx.TranID, err)
		}
	}

	// Full-field Get assertion.
	got, err := s.Get(ctx, "T0000000000000001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TranID != t1.TranID || got.TranTypeCode != t1.TranTypeCode ||
		got.TranCatCode != t1.TranCatCode || got.TranSource != t1.TranSource ||
		got.TranDesc != t1.TranDesc || !got.TranAmt.Equal(t1.TranAmt) ||
		got.TranMerchantID != t1.TranMerchantID || got.TranMerchantName != t1.TranMerchantName ||
		got.TranMerchantCity != t1.TranMerchantCity || got.TranMerchantZip != t1.TranMerchantZip ||
		got.TranCardNum != t1.TranCardNum || got.TranOrigTS != t1.TranOrigTS || got.TranProcTS != t1.TranProcTS {
		t.Fatalf("get mismatch:\nwant %+v\ngot  %+v", t1, got)
	}

	// GetByCard: count and content — each result must have TranCardNum="4111...".
	byCard, err := s.GetByCard(ctx, "4111111111111111", 0)
	if err != nil {
		t.Fatalf("get-by-card: %v", err)
	}
	if len(byCard) != 2 {
		t.Fatalf("get-by-card: want 2, got %d", len(byCard))
	}
	for _, tx := range byCard {
		if tx.TranCardNum != "4111111111111111" {
			t.Fatalf("get-by-card: wrong card %q in result %+v", tx.TranCardNum, tx)
		}
	}

	// GetByCard with limit=1 returns only 1 row.
	limited, err := s.GetByCard(ctx, "4111111111111111", 1)
	if err != nil {
		t.Fatalf("get-by-card limited: %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("get-by-card limited: want 1, got %d", len(limited))
	}

	// Update and verify.
	got.TranAmt = decimal.RequireFromString("100.00")
	got.TranDesc = "REFUND"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := s.Get(ctx, "T0000000000000001")
	if !reread.TranAmt.Equal(decimal.RequireFromString("100.00")) || reread.TranDesc != "REFUND" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update of missing row returns ErrNotFound.
	if err := s.Update(ctx, sampleTransaction("TXXXXXXXXXXXXXXXXX", "card")); !errors.Is(err, repo.ErrNotFound) {
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

	// Browse cursor: start from T000...002 → 2 results.
	page, err := s.Browse(ctx, "T0000000000000002", 10)
	if err != nil {
		t.Fatalf("browse cursor: %v", err)
	}
	if len(page) != 2 || page[0].TranID != "T0000000000000002" {
		t.Fatalf("browse cursor: unexpected %v", page)
	}

	// Delete.
	if err := s.Delete(ctx, "T0000000000000003"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "T0000000000000003"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}

	// Delete of missing row returns ErrNotFound.
	if err := s.Delete(ctx, "T0000000000000003"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}

	// Duplicate Create returns ErrConflict.
	if err := s.Create(ctx, sampleTransaction("T0000000000000001", "card")); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}
}

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

	if err := s.Create(ctx, sampleTransaction("T0000000000000001", "4111111111111111")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, sampleTransaction("T0000000000000002", "4111111111111111")); err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if err := s.Create(ctx, sampleTransaction("T0000000000000003", "4222222222222222")); err != nil {
		t.Fatalf("create 3: %v", err)
	}

	got, err := s.Get(ctx, "T0000000000000001")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.TranAmt.Equal(decimal.RequireFromString("42.50")) {
		t.Fatalf("get mismatch: %+v", got)
	}

	byCard, err := s.GetByCard(ctx, "4111111111111111", 10)
	if err != nil {
		t.Fatalf("get-by-card: %v", err)
	}
	if len(byCard) != 2 {
		t.Fatalf("get-by-card: want 2, got %d", len(byCard))
	}

	got.TranAmt = decimal.RequireFromString("100.00")
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := s.Get(ctx, "T0000000000000001")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if !reread.TranAmt.Equal(decimal.RequireFromString("100.00")) {
		t.Fatalf("update not persisted: %+v", reread)
	}

	all, err := s.Browse(ctx, "", 10)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("browse: want 3, got %d", len(all))
	}

	if err := s.Delete(ctx, "T0000000000000003"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "T0000000000000003"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("get deleted: want ErrNotFound, got %v", err)
	}
}

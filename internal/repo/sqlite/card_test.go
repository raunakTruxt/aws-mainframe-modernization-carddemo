package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
)

func sampleCard(num string, acctID int64) *domain.CardRecord {
	return &domain.CardRecord{
		CardNum:            num,
		CardAcctID:         acctID,
		CardCVVCode:        123,
		CardEmbossedName:   "JANE DOE",
		CardExpirationDate: "2025-12-31",
		CardActiveStatus:   "Y",
	}
}

func TestCardStore(t *testing.T) {
	ctx := context.Background()
	s := NewCardStore(newTestDB(t))

	if err := s.Create(ctx, sampleCard("4111111111111111", 100)); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(ctx, sampleCard("4222222222222222", 100)); err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if err := s.Create(ctx, sampleCard("4333333333333333", 200)); err != nil {
		t.Fatalf("create 3: %v", err)
	}

	got, err := s.Get(ctx, "4111111111111111")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CardAcctID != 100 {
		t.Fatalf("get mismatch: %+v", got)
	}

	byAcct, err := s.GetByAccount(ctx, 100)
	if err != nil {
		t.Fatalf("get-by-account: %v", err)
	}
	if len(byAcct) != 2 {
		t.Fatalf("get-by-account: want 2, got %d", len(byAcct))
	}

	got.CardActiveStatus = "N"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, err := s.Get(ctx, "4111111111111111")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if reread.CardActiveStatus != "N" {
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

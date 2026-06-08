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

	c1 := sampleCard("4111111111111111", 100)
	c2 := sampleCard("4222222222222222", 100)
	c3 := sampleCard("4333333333333333", 200)

	for _, c := range []*domain.CardRecord{c1, c2, c3} {
		if err := s.Create(ctx, c); err != nil {
			t.Fatalf("create %s: %v", c.CardNum, err)
		}
	}

	// Full-field Get assertion.
	got, err := s.Get(ctx, "4111111111111111")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CardNum != c1.CardNum || got.CardAcctID != c1.CardAcctID ||
		got.CardCVVCode != c1.CardCVVCode || got.CardEmbossedName != c1.CardEmbossedName ||
		got.CardExpirationDate != c1.CardExpirationDate || got.CardActiveStatus != c1.CardActiveStatus {
		t.Fatalf("get mismatch:\nwant %+v\ngot  %+v", c1, got)
	}

	// GetByAccount: count and content — each returned card must have acctID=100.
	byAcct, err := s.GetByAccount(ctx, 100)
	if err != nil {
		t.Fatalf("get-by-account: %v", err)
	}
	if len(byAcct) != 2 {
		t.Fatalf("get-by-account: want 2, got %d", len(byAcct))
	}
	for _, c := range byAcct {
		if c.CardAcctID != 100 {
			t.Fatalf("get-by-account: returned card %q has wrong acct %d", c.CardNum, c.CardAcctID)
		}
	}

	// Update and verify.
	got.CardActiveStatus = "N"
	got.CardEmbossedName = "JOHN DOE"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	reread, _ := s.Get(ctx, "4111111111111111")
	if reread.CardActiveStatus != "N" || reread.CardEmbossedName != "JOHN DOE" {
		t.Fatalf("update not persisted: %+v", reread)
	}

	// Update of missing row returns ErrNotFound.
	if err := s.Update(ctx, sampleCard("9999999999999999", 0)); !errors.Is(err, repo.ErrNotFound) {
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

	// Browse cursor: start from "4222..." → should see 4222 and 4333.
	page, err := s.Browse(ctx, "4222222222222222", 10)
	if err != nil {
		t.Fatalf("browse cursor: %v", err)
	}
	if len(page) != 2 || page[0].CardNum != "4222222222222222" || page[1].CardNum != "4333333333333333" {
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
	if err := s.Create(ctx, sampleCard("4111111111111111", 100)); !errors.Is(err, repo.ErrConflict) {
		t.Fatalf("duplicate create: want ErrConflict, got %v", err)
	}
}

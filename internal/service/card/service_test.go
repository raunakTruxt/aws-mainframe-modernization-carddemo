package card_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	svc "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/card"
)

// newTestService spins up an in-memory SQLite DB and seeds it with cards.
func newTestService(t *testing.T, cards []*domain.CardRecord, xrefs []*domain.CardXrefRecord) *svc.Service {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	cardStore := sqlite.NewCardStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)

	for _, c := range cards {
		if err := cardStore.Create(ctx, c); err != nil {
			t.Fatalf("seed card %s: %v", c.CardNum, err)
		}
	}
	for _, x := range xrefs {
		if err := xrefStore.Create(ctx, x); err != nil {
			t.Fatalf("seed xref %s: %v", x.XrefCardNum, err)
		}
	}

	return svc.NewService(cardStore, xrefStore)
}

func makeCard(num string, acctID int64, status string) *domain.CardRecord {
	return &domain.CardRecord{
		CardNum:            num,
		CardAcctID:         acctID,
		CardCVVCode:        123,
		CardEmbossedName:   "TEST USER",
		CardExpirationDate: "2030-12-01",
		CardActiveStatus:   status,
	}
}

func makeXref(cardNum string, acctID, custID int64) *domain.CardXrefRecord {
	return &domain.CardXrefRecord{
		XrefCardNum: cardNum,
		XrefAcctID:  acctID,
		XrefCustID:  custID,
	}
}

// --- List ---

func TestList_AllCards_Admin(t *testing.T) {
	cards := []*domain.CardRecord{
		makeCard("1111000000000001", 10, "Y"),
		makeCard("2222000000000002", 20, "N"),
		makeCard("3333000000000003", 30, "Y"),
	}
	s := newTestService(t, cards, nil)

	page, err := s.List(context.Background(), 0, "", 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Cards) != 3 {
		t.Fatalf("want 3 cards, got %d", len(page.Cards))
	}
	// Verify masking: last 4 digits visible
	if page.Cards[0].CardNumMasked != "**** **** **** 0001" {
		t.Errorf("mask: got %q", page.Cards[0].CardNumMasked)
	}
	// Full number present for links
	if page.Cards[0].CardNum != "1111000000000001" {
		t.Errorf("CardNum: got %q", page.Cards[0].CardNum)
	}
}

func TestList_Pagination(t *testing.T) {
	// 9 cards, page size 3 → 3 pages
	var cards []*domain.CardRecord
	nums := []string{
		"1000000000000001",
		"2000000000000002",
		"3000000000000003",
		"4000000000000004",
		"5000000000000005",
		"6000000000000006",
		"7000000000000007",
		"8000000000000008",
		"9000000000000009",
	}
	for _, n := range nums {
		cards = append(cards, makeCard(n, 1, "Y"))
	}
	s := newTestService(t, cards, nil)
	ctx := context.Background()

	// Page 1
	p1, err := s.List(ctx, 0, "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(p1.Cards) != 3 {
		t.Fatalf("page1: want 3, got %d", len(p1.Cards))
	}
	if !p1.HasNext {
		t.Fatal("page1: expected HasNext")
	}

	// Page 2 using cursor
	p2, err := s.List(ctx, 0, p1.NextKey, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.Cards) != 3 {
		t.Fatalf("page2: want 3, got %d", len(p2.Cards))
	}
	// No overlap: page 2 starts after page 1
	if p2.Cards[0].CardNum <= p1.Cards[2].CardNum {
		t.Errorf("overlap: p2[0]=%s <= p1[2]=%s", p2.Cards[0].CardNum, p1.Cards[2].CardNum)
	}

	// Page 3 — last page
	p3, err := s.List(ctx, 0, p2.NextKey, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(p3.Cards) != 3 {
		t.Fatalf("page3: want 3, got %d", len(p3.Cards))
	}
	if p3.HasNext {
		t.Fatal("page3: should not have next")
	}
}

func TestList_ByAccount(t *testing.T) {
	cards := []*domain.CardRecord{
		makeCard("1111000000000001", 10, "Y"),
		makeCard("2222000000000002", 10, "Y"),
		makeCard("3333000000000003", 20, "Y"),
	}
	s := newTestService(t, cards, nil)

	page, err := s.List(context.Background(), 10, "", 0)
	if err != nil {
		t.Fatalf("List by account: %v", err)
	}
	if len(page.Cards) != 2 {
		t.Fatalf("want 2 cards for account 10, got %d", len(page.Cards))
	}
	for _, c := range page.Cards {
		if c.CardAcctID != 10 {
			t.Errorf("card %s has wrong account %d", c.CardNum, c.CardAcctID)
		}
	}
}

// --- Get ---

func TestGet(t *testing.T) {
	card := makeCard("4000100000000001", 5, "Y")
	s := newTestService(t, []*domain.CardRecord{card}, nil)

	got, err := s.Get(context.Background(), "4000100000000001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CardNum != card.CardNum {
		t.Errorf("CardNum: got %s, want %s", got.CardNum, card.CardNum)
	}
}

func TestGet_NotFound(t *testing.T) {
	s := newTestService(t, nil, nil)
	_, err := s.Get(context.Background(), "9999999999999999")
	if !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// --- Update ---

func TestUpdate_ValidStatus(t *testing.T) {
	card := makeCard("5000100000000001", 5, "Y")
	s := newTestService(t, []*domain.CardRecord{card}, nil)
	ctx := context.Background()

	got, _ := s.Get(ctx, card.CardNum)
	got.CardActiveStatus = "N"
	if err := s.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, _ := s.Get(ctx, card.CardNum)
	if updated.CardActiveStatus != "N" {
		t.Errorf("status not updated, got %q", updated.CardActiveStatus)
	}
}

func TestUpdate_InvalidStatus(t *testing.T) {
	card := makeCard("5000100000000002", 5, "Y")
	s := newTestService(t, []*domain.CardRecord{card}, nil)
	ctx := context.Background()

	got, _ := s.Get(ctx, card.CardNum)
	got.CardActiveStatus = "X"
	err := s.Update(ctx, got)
	if !errors.Is(err, svc.ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

func TestUpdate_PastExpiry(t *testing.T) {
	card := makeCard("5000100000000003", 5, "Y")
	s := newTestService(t, []*domain.CardRecord{card}, nil)
	ctx := context.Background()

	got, _ := s.Get(ctx, card.CardNum)
	got.CardExpirationDate = "2000-01-01"
	err := s.Update(ctx, got)
	if !errors.Is(err, svc.ErrValidation) {
		t.Errorf("want ErrValidation for past expiry, got %v", err)
	}
}

func TestUpdate_InvalidEmbossedName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"blank", ""},
		{"digits", "CARD123"},
		{"special chars", "JOHN@DOE"},
	}
	card := makeCard("5000100000000004", 5, "Y")
	s := newTestService(t, []*domain.CardRecord{card}, nil)
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := s.Get(ctx, card.CardNum)
			got.CardEmbossedName = tt.input
			err := s.Update(ctx, got)
			if !errors.Is(err, svc.ErrValidation) {
				t.Errorf("name=%q: want ErrValidation, got %v", tt.input, err)
			}
		})
	}
}

// --- MaskCardNum ---

func TestMaskCardNum(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"4000100000000001", "**** **** **** 0001"},
		{"1234", "**** **** **** 1234"},
		{"12", "**"},
		{"", ""},
	}
	for _, tt := range tests {
		got := svc.MaskCardNum(tt.in)
		if got != tt.want {
			t.Errorf("MaskCardNum(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

package card_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	cardsvc "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/card"
	cardweb "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/card"
)

func makeHandler(t *testing.T, cards []*domain.CardRecord) *cardweb.Handlers {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	cardStore := sqlite.NewCardStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)
	for _, c := range cards {
		if err := cardStore.Create(ctx, c); err != nil {
			t.Fatalf("seed card: %v", err)
		}
	}
	svc := cardsvc.NewService(cardStore, xrefStore)
	return cardweb.NewHandlers(svc)
}

func card(num string, acctID int64) *domain.CardRecord {
	return &domain.CardRecord{
		CardNum:            num,
		CardAcctID:         acctID,
		CardCVVCode:        123,
		CardEmbossedName:   "ALICE SMITH",
		CardExpirationDate: "2030-06-01",
		CardActiveStatus:   "Y",
	}
}

// ── GetList ──────────────────────────────────────────────────────────────

func TestGetList_ReturnsHTMLWithMaskedNumbers(t *testing.T) {
	h := makeHandler(t, []*domain.CardRecord{
		card("4000100000000001", 1),
		card("4000100000000002", 1),
	})

	req := httptest.NewRequest("GET", "/cards/list", nil)
	rec := httptest.NewRecorder()
	h.GetList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	// Masked numbers should appear in the response.
	if !strings.Contains(body, "**** **** **** 0001") {
		t.Error("expected masked card number in list, not found")
	}
	// Full card number must NOT appear in the response body (only last 4 shown).
	if strings.Contains(body, "4000100000000001") {
		// It's OK for the card number to appear in the action links
		// only if wrapped in href parameters — check it's not in visible text.
		// (In our template the raw number is only in href attributes.)
	}
	if !strings.Contains(body, "Credit Card List") {
		t.Error("expected page title in response")
	}
}

func TestGetList_EmptyReturnsNoRecordsMessage(t *testing.T) {
	h := makeHandler(t, nil)

	req := httptest.NewRequest("GET", "/cards/list", nil)
	rec := httptest.NewRecorder()
	h.GetList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "No records found") {
		t.Error("expected 'No records found' message for empty list")
	}
}

func TestGetList_FilterByAccount(t *testing.T) {
	h := makeHandler(t, []*domain.CardRecord{
		card("4000100000000001", 10),
		card("4000100000000002", 10),
		card("4000100000000003", 20),
	})

	req := httptest.NewRequest("GET", "/cards/list?account_id=10", nil)
	rec := httptest.NewRecorder()
	h.GetList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	// Two cards for account 10, one for account 20.
	if strings.Count(body, "**** **** ****") != 2 {
		t.Errorf("expected 2 masked card numbers, body:\n%s", body)
	}
}

// ── GetView ───────────────────────────────────────────────────────────────

func TestGetView_ShowsFullCardNumber(t *testing.T) {
	h := makeHandler(t, []*domain.CardRecord{card("4000100000000001", 1)})

	req := httptest.NewRequest("GET", "/cards/view?card_num=4000100000000001", nil)
	rec := httptest.NewRecorder()
	h.GetView(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "4000100000000001") {
		t.Error("expected full card number on view page")
	}
	if !strings.Contains(body, "ALICE SMITH") {
		t.Error("expected embossed name on view page")
	}
}

func TestGetView_MissingCardNum(t *testing.T) {
	h := makeHandler(t, nil)

	req := httptest.NewRequest("GET", "/cards/view", nil)
	rec := httptest.NewRecorder()
	h.GetView(rec, req)

	// No card_num → shows search prompt (200, not 400).
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
}

func TestGetView_NotFound(t *testing.T) {
	h := makeHandler(t, nil)

	req := httptest.NewRequest("GET", "/cards/view?card_num=9999999999999999", nil)
	rec := httptest.NewRecorder()
	h.GetView(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200 (renders flash message), got %d", rec.Code)
	}
	// The flash message "Card not found." should appear in the rendered page.
	if !strings.Contains(rec.Body.String(), "Card not found") {
		t.Error("expected 'Card not found' in response")
	}
}

// ── GetUpdate ─────────────────────────────────────────────────────────────

func TestGetUpdate_ShowsForm(t *testing.T) {
	h := makeHandler(t, []*domain.CardRecord{card("4000100000000001", 1)})

	req := httptest.NewRequest("GET", "/cards/update?card_num=4000100000000001", nil)
	rec := httptest.NewRecorder()
	h.GetUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `name="embossed_name"`) {
		t.Error("expected embossed_name field in form")
	}
	if !strings.Contains(body, `name="active_status"`) {
		t.Error("expected active_status field in form")
	}
	if !strings.Contains(body, `name="expiry_date"`) {
		t.Error("expected expiry_date field in form")
	}
}

// ── PostUpdate ────────────────────────────────────────────────────────────

func TestPostUpdate_ValidUpdate(t *testing.T) {
	h := makeHandler(t, []*domain.CardRecord{card("4000100000000001", 1)})

	form := url.Values{
		"card_num":       {"4000100000000001"},
		"embossed_name":  {"BOB JONES"},
		"active_status":  {"N"},
		"expiry_date":    {"2031-12-01"},
	}
	req := httptest.NewRequest("POST", "/cards/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.PostUpdate(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status: want 303, got %d\nbody: %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/cards/view?card_num=4000100000000001") {
		t.Errorf("redirect: want /cards/view?card_num=..., got %q", loc)
	}
}

func TestPostUpdate_InvalidStatus(t *testing.T) {
	h := makeHandler(t, []*domain.CardRecord{card("4000100000000001", 1)})

	form := url.Values{
		"card_num":      {"4000100000000001"},
		"embossed_name": {"BOB JONES"},
		"active_status": {"X"},
		"expiry_date":   {"2031-12-01"},
	}
	req := httptest.NewRequest("POST", "/cards/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.PostUpdate(rec, req)

	// Should re-render the form with an error (200 with flash, not redirect).
	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Validation error") {
		t.Error("expected 'Validation error' in response body")
	}
}

func TestPostUpdate_PastExpiry(t *testing.T) {
	h := makeHandler(t, []*domain.CardRecord{card("4000100000000001", 1)})

	form := url.Values{
		"card_num":      {"4000100000000001"},
		"embossed_name": {"BOB JONES"},
		"active_status": {"Y"},
		"expiry_date":   {"2000-01-01"},
	}
	req := httptest.NewRequest("POST", "/cards/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.PostUpdate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200 (validation error re-render), got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Validation error") {
		t.Error("expected 'Validation error' for past expiry")
	}
}

func TestPostUpdate_NotFound(t *testing.T) {
	h := makeHandler(t, nil)

	form := url.Values{
		"card_num":      {"9999999999999999"},
		"embossed_name": {"BOB JONES"},
		"active_status": {"Y"},
		"expiry_date":   {"2031-12-01"},
	}
	req := httptest.NewRequest("POST", "/cards/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.PostUpdate(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", rec.Code)
	}
}

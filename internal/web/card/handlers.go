// Package card provides HTTP handlers for the credit-card screens:
//   - GET  /cards/list         → COCRDLIC (paginated list)
//   - GET  /cards/view         → COCRDSLC (detail view)
//   - GET  /cards/update       → COCRDUPC (update form)
//   - POST /cards/update       → COCRDUPC (update submit)
package card

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	cardsvc "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/card"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/layout"
)

// Handlers groups the card HTTP handlers.
type Handlers struct {
	svc *cardsvc.Service
}

// NewHandlers returns a Handlers wired to the given Service.
func NewHandlers(svc *cardsvc.Service) *Handlers {
	return &Handlers{svc: svc}
}

// ── List (COCRDLIC) ────────────────────────────────────────────────────────

type listBody struct {
	Cards     []*cardsvc.CardSummary
	NextKey   string
	HasNext   bool
	PrevKeys  []string // stack of previous page start-keys (for a "back" link)
	AccountID int64
	StartKey  string
	CSRFToken string
}

const listHTML = `{{define "content"}}
<section class="card-list">
  <h2>Credit Card List</h2>

  <form method="get" action="/cards/list" class="filter-form">
    <label>Account ID:
      <input type="number" name="account_id" value="{{.Body.AccountID}}" min="0">
    </label>
    <button type="submit">Filter</button>
  </form>

  {{if .Body.Cards}}
  <table class="data-table">
    <thead>
      <tr>
        <th>Card Number</th>
        <th>Account ID</th>
        <th>Embossed Name</th>
        <th>Expiry</th>
        <th>Status</th>
        <th>Actions</th>
      </tr>
    </thead>
    <tbody>
      {{range .Body.Cards}}
      <tr>
        <td class="mono">{{.CardNumMasked}}</td>
        <td>{{.CardAcctID}}</td>
        <td>{{.CardEmbossedName}}</td>
        <td>{{.CardExpirationDate}}</td>
        <td>{{if eq .CardActiveStatus "Y"}}Active{{else}}Inactive{{end}}</td>
        <td>
          <a href="/cards/view?card_num={{.CardNum}}">View</a>
          &nbsp;|&nbsp;
          <a href="/cards/update?card_num={{.CardNum}}">Update</a>
        </td>
      </tr>
      {{end}}
    </tbody>
  </table>

  <div class="pagination">
    {{if .Body.HasNext}}
    <a href="/cards/list?start={{.Body.NextKey}}&account_id={{.Body.AccountID}}">Next &rarr;</a>
    {{end}}
  </div>
  {{else}}
  <p class="info-msg">No records found for this search condition.</p>
  {{end}}

  <p><a href="/">&larr; Main Menu</a></p>
</section>
{{end}}`

// GetList handles GET /cards/list.
// Query params: account_id (0 = all / admin), start (cursor key).
func (h *Handlers) GetList(w http.ResponseWriter, r *http.Request) {
	acctID, _ := strconv.ParseInt(r.URL.Query().Get("account_id"), 10, 64)
	startKey := r.URL.Query().Get("start")

	page, err := h.svc.List(r.Context(), acctID, startKey, 0)
	if err != nil {
		http.Error(w, "error listing cards: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pd := layout.NewPage(r, "Credit Card List").WithBody(listBody{
		Cards:     page.Cards,
		NextKey:   page.NextKey,
		HasNext:   page.HasNext,
		AccountID: acctID,
		StartKey:  startKey,
	})
	layout.Render(w, pd, listHTML)
}

// ── View (COCRDSLC) ───────────────────────────────────────────────────────

type viewBody struct {
	Card *domain.CardRecord
}

const viewHTML = `{{define "content"}}
<section class="card-view">
  <h2>Credit Card View</h2>

  <form method="get" action="/cards/view" class="filter-form">
    <label>Card Number:
      <input type="text" name="card_num" maxlength="16"
             value="{{if .Body.Card}}{{.Body.Card.CardNum}}{{end}}">
    </label>
    <button type="submit">Search</button>
  </form>

  {{if .Flash}}<p class="error-msg">{{.Flash}}</p>{{end}}

  {{if .Body.Card}}
  <dl class="detail-list">
    <dt>Card Number</dt>      <dd class="mono">{{.Body.Card.CardNum}}</dd>
    <dt>Account ID</dt>       <dd>{{.Body.Card.CardAcctID}}</dd>
    <dt>CVV</dt>              <dd>{{.Body.Card.CardCVVCode}}</dd>
    <dt>Embossed Name</dt>    <dd>{{.Body.Card.CardEmbossedName}}</dd>
    <dt>Expiration Date</dt>  <dd>{{.Body.Card.CardExpirationDate}}</dd>
    <dt>Active Status</dt>    <dd>{{if eq .Body.Card.CardActiveStatus "Y"}}Active{{else}}Inactive{{end}}</dd>
  </dl>
  <p>
    <a href="/cards/update?card_num={{.Body.Card.CardNum}}">Update this card</a>
    &nbsp;|&nbsp;
    <a href="/cards/list">Back to list</a>
  </p>
  {{else if not .Flash}}
  <p class="info-msg">Please enter Account and Card Number.</p>
  <p><a href="/cards/list">Back to list</a></p>
  {{end}}
</section>
{{end}}`

// GetView handles GET /cards/view?card_num=.
// If card_num is absent, a search prompt is displayed (matching the COCRDSLC
// initial state where the user has not yet entered a card number).
func (h *Handlers) GetView(w http.ResponseWriter, r *http.Request) {
	cardNum := strings.TrimSpace(r.URL.Query().Get("card_num"))
	if cardNum == "" {
		pd := layout.NewPage(r, "Credit Card View").WithBody(viewBody{})
		layout.Render(w, pd, viewHTML)
		return
	}

	card, err := h.svc.Get(r.Context(), cardNum)
	if errors.Is(err, repo.ErrNotFound) {
		pd := layout.NewPage(r, "Credit Card View").
			WithFlash("Card not found.").
			WithBody(viewBody{})
		layout.Render(w, pd, viewHTML)
		return
	}
	if err != nil {
		http.Error(w, "error fetching card: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pd := layout.NewPage(r, "Credit Card View").WithBody(viewBody{Card: card})
	layout.Render(w, pd, viewHTML)
}

// ── Update (COCRDUPC) ─────────────────────────────────────────────────────

type updateBody struct {
	Card      *domain.CardRecord
	CSRFToken string
}

const updateHTML = `{{define "content"}}
<section class="card-update">
  <h2>Credit Card Update</h2>

  {{if not .Body.Card}}
  <form method="get" action="/cards/update" class="filter-form">
    <label>Card Number:
      <input type="text" name="card_num" maxlength="16">
    </label>
    <button type="submit">Search</button>
  </form>
  {{if .Flash}}<p class="error-msg">{{.Flash}}</p>{{end}}
  <p><a href="/cards/list">Back to list</a></p>
  {{else}}
  <form method="post" action="/cards/update" novalidate>
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <input type="hidden" name="card_num" value="{{.Body.Card.CardNum}}">

    <dl class="detail-list">
      <dt>Card Number</dt>   <dd class="mono">{{.Body.Card.CardNum}}</dd>
      <dt>Account ID</dt>    <dd>{{.Body.Card.CardAcctID}}</dd>
    </dl>

    <div class="field">
      <label for="embossed_name">Embossed Name (letters and spaces only):</label>
      <input id="embossed_name" name="embossed_name" type="text"
             maxlength="50" value="{{.Body.Card.CardEmbossedName}}" required>
    </div>

    <div class="field">
      <label for="active_status">Active Status (Y / N):</label>
      <input id="active_status" name="active_status" type="text"
             maxlength="1" value="{{.Body.Card.CardActiveStatus}}" required>
    </div>

    <div class="field">
      <label>Expiry (YYYY-MM-DD):</label>
      <input name="expiry_date" type="text" maxlength="10"
             value="{{.Body.Card.CardExpirationDate}}" required
             placeholder="YYYY-MM-DD">
    </div>

    <div class="form-actions">
      <button type="submit">Save (F5)</button>
      <a href="/cards/view?card_num={{.Body.Card.CardNum}}">Cancel (F3)</a>
    </div>
  </form>
  {{end}}
</section>
{{end}}`

// GetUpdate handles GET /cards/update?card_num=.
// If card_num is absent, a search prompt is shown (matching COCRDUPC initial state).
func (h *Handlers) GetUpdate(w http.ResponseWriter, r *http.Request) {
	cardNum := strings.TrimSpace(r.URL.Query().Get("card_num"))
	if cardNum == "" {
		pd := layout.NewPage(r, "Credit Card Update").WithBody(updateBody{})
		layout.Render(w, pd, updateHTML)
		return
	}

	card, err := h.svc.Get(r.Context(), cardNum)
	if errors.Is(err, repo.ErrNotFound) {
		pd := layout.NewPage(r, "Credit Card Update").
			WithFlash("Card not found.").
			WithBody(updateBody{})
		layout.Render(w, pd, updateHTML)
		return
	}
	if err != nil {
		http.Error(w, "error fetching card: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pd := layout.NewPage(r, "Credit Card Update").WithBody(updateBody{Card: card})
	layout.Render(w, pd, updateHTML)
}

// PostUpdate handles POST /cards/update.
// Form fields: card_num (hidden), embossed_name, active_status, expiry_date.
func (h *Handlers) PostUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	cardNum := strings.TrimSpace(r.FormValue("card_num"))
	if cardNum == "" {
		http.Error(w, "card_num is required", http.StatusBadRequest)
		return
	}

	card, err := h.svc.Get(r.Context(), cardNum)
	if errors.Is(err, repo.ErrNotFound) {
		http.Error(w, "card not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "error fetching card: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Apply form values to the card record.
	card.CardEmbossedName = strings.TrimSpace(r.FormValue("embossed_name"))
	card.CardActiveStatus = strings.ToUpper(strings.TrimSpace(r.FormValue("active_status")))
	card.CardExpirationDate = strings.TrimSpace(r.FormValue("expiry_date"))

	if err := h.svc.Update(r.Context(), card); err != nil {
		if errors.Is(err, cardsvc.ErrValidation) {
			// Re-render the form with the validation message.
			msg := strings.TrimPrefix(err.Error(), "validation error: ")
			pd := layout.NewPage(r, "Credit Card Update").
				WithFlash(fmt.Sprintf("Validation error: %s", msg)).
				WithBody(updateBody{Card: card})
			layout.Render(w, pd, updateHTML)
			return
		}
		http.Error(w, "update failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Success: redirect to view screen.
	http.Redirect(w, r, fmt.Sprintf("/cards/view?card_num=%s&flash=Changes+committed+to+database", cardNum), http.StatusSeeOther)
}

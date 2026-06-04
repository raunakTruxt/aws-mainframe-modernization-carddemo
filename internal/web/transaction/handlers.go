// Package transaction is the HTTP edge for the CardDemo online transaction
// screens. It is the Go replacement for the COBOL programs:
//
//	COTRN00C  -> GET  /transactions            (list by card or browse all)
//	COTRN01C  -> GET  /transactions/view       (view one transaction)
//	COTRN02C  -> GET  /transactions/add        (add form)
//	           -> POST /transactions/add        (submit add)
//	COBIL00C  -> GET  /billing                 (bill payment form)
//	           -> POST /billing                 (submit payment)
package transaction

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	svc "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/transaction"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/layout"
	"github.com/shopspring/decimal"
)

const pageSize = 10

// Handlers exposes the HTTP entry points for transaction screens.
type Handlers struct {
	Service *svc.Service
}

// ─── Transaction List (COTRN00C) ─────────────────────────────────────────────

type listBody struct {
	Txns      []*domain.TransactionRecord
	CardNum   string
	StartID   string
	NextStart string
	CSRFToken string
}

const listContent = `{{define "content"}}
<section class="txn-list-screen">
  <form method="get" action="/transactions" class="search-form">
    <label for="card">Card Number:</label>
    <input id="card" name="card" value="{{.Body.CardNum}}" maxlength="16" size="20">
    <button type="submit">Search</button>
    {{if .Body.CardNum}}
    <a href="/transactions">Clear</a>
    {{end}}
  </form>
  {{if .Body.Txns}}
  <table class="txn-table">
    <thead>
      <tr>
        <th>Tran ID</th>
        <th>Type</th>
        <th>Date</th>
        <th>Amount</th>
        <th>Card</th>
        <th>Description</th>
      </tr>
    </thead>
    <tbody>
      {{range .Body.Txns}}
      <tr>
        <td><a href="/transactions/view?id={{.TranID}}">{{.TranID}}</a></td>
        <td>{{.TranTypeCode}}</td>
        <td>{{.TranOrigTS}}</td>
        <td class="amt">{{.TranAmt}}</td>
        <td>{{.TranCardNum}}</td>
        <td>{{.TranDesc}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{if .Body.NextStart}}
  <div class="pager">
    <a href="/transactions?card={{.Body.CardNum}}&start={{.Body.NextStart}}">Next Page &rarr;</a>
  </div>
  {{end}}
  {{else}}
  <p class="no-data">No transactions found.</p>
  {{end}}
  <p><a href="/">&#8592; Main Menu</a></p>
</section>
{{end}}`

// ListTxns serves COTRN00C — the transaction list screen.
// Query params: card (optional card filter), start (pagination cursor).
func (h *Handlers) ListTxns(w http.ResponseWriter, r *http.Request) {
	cardNum := strings.TrimSpace(r.URL.Query().Get("card"))
	startID := strings.TrimSpace(r.URL.Query().Get("start"))

	// Fetch one extra row to detect whether there is a next page.
	txns, err := h.Service.List(r.Context(), cardNum, startID, pageSize+1)
	if err != nil {
		renderFlash(w, r, "Transaction List", fmt.Sprintf("Error loading transactions: %v", err), listContent, listBody{CardNum: cardNum, CSRFToken: csrfToken(r)})
		return
	}

	var nextStart string
	if len(txns) > pageSize {
		nextStart = txns[pageSize].TranID
		txns = txns[:pageSize]
	}

	pd := layout.NewPage(r, "Transaction List").WithBody(listBody{
		Txns:      txns,
		CardNum:   cardNum,
		StartID:   startID,
		NextStart: nextStart,
		CSRFToken: csrfToken(r),
	})
	layout.Render(w, pd, listContent)
}

// ─── Transaction View (COTRN01C) ─────────────────────────────────────────────

type viewBody struct {
	Txn *domain.TransactionRecord
}

const viewContent = `{{define "content"}}
<section class="txn-view-screen">
  {{if .Body.Txn}}
  <table class="detail-table">
    <tr><th>Tran ID</th><td>{{.Body.Txn.TranID}}</td></tr>
    <tr><th>Type Code</th><td>{{.Body.Txn.TranTypeCode}}</td></tr>
    <tr><th>Category</th><td>{{.Body.Txn.TranCatCode}}</td></tr>
    <tr><th>Source</th><td>{{.Body.Txn.TranSource}}</td></tr>
    <tr><th>Description</th><td>{{.Body.Txn.TranDesc}}</td></tr>
    <tr><th>Amount</th><td>{{.Body.Txn.TranAmt}}</td></tr>
    <tr><th>Card Number</th><td>{{.Body.Txn.TranCardNum}}</td></tr>
    <tr><th>Merchant ID</th><td>{{.Body.Txn.TranMerchantID}}</td></tr>
    <tr><th>Merchant Name</th><td>{{.Body.Txn.TranMerchantName}}</td></tr>
    <tr><th>Merchant City</th><td>{{.Body.Txn.TranMerchantCity}}</td></tr>
    <tr><th>Merchant Zip</th><td>{{.Body.Txn.TranMerchantZip}}</td></tr>
    <tr><th>Orig Timestamp</th><td>{{.Body.Txn.TranOrigTS}}</td></tr>
    <tr><th>Proc Timestamp</th><td>{{.Body.Txn.TranProcTS}}</td></tr>
  </table>
  {{else}}
  <p class="no-data">Transaction not found.</p>
  {{end}}
  <p><a href="/transactions">&#8592; Transaction List</a></p>
</section>
{{end}}`

// ViewTxn serves COTRN01C — the transaction detail screen.
// Query param: id (required transaction ID).
func (h *Handlers) ViewTxn(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		http.Redirect(w, r, "/transactions", http.StatusSeeOther)
		return
	}

	txn, err := h.Service.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			pd := layout.NewPage(r, "Transaction View").WithFlash("Transaction not found.").WithBody(viewBody{})
			layout.Render(w, pd, viewContent)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	pd := layout.NewPage(r, "Transaction View").WithBody(viewBody{Txn: txn})
	layout.Render(w, pd, viewContent)
}

// ─── Transaction Add (COTRN02C) ──────────────────────────────────────────────

type addBody struct {
	CardNum      string
	TranTypeCode string
	TranCatCode  string
	TranSource   string
	TranDesc     string
	TranAmt      string
	MerchantID   string
	MerchantName string
	MerchantCity string
	MerchantZip  string
	CSRFToken    string
}

const addContent = `{{define "content"}}
<section class="txn-add-screen">
  <form method="post" action="/transactions/add">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <table class="form-table">
      <tr>
        <th><label for="card_num">Card Number</label></th>
        <td><input id="card_num" name="card_num" value="{{.Body.CardNum}}" maxlength="16" size="20" required></td>
      </tr>
      <tr>
        <th><label for="tran_type">Tran Type</label></th>
        <td><input id="tran_type" name="tran_type" value="{{.Body.TranTypeCode}}" maxlength="2" size="4" required></td>
      </tr>
      <tr>
        <th><label for="tran_cat">Tran Category</label></th>
        <td><input id="tran_cat" name="tran_cat" value="{{.Body.TranCatCode}}" maxlength="4" size="6" required></td>
      </tr>
      <tr>
        <th><label for="tran_source">Source</label></th>
        <td><input id="tran_source" name="tran_source" value="{{.Body.TranSource}}" maxlength="10" size="12"></td>
      </tr>
      <tr>
        <th><label for="tran_desc">Description</label></th>
        <td><input id="tran_desc" name="tran_desc" value="{{.Body.TranDesc}}" maxlength="100" size="40"></td>
      </tr>
      <tr>
        <th><label for="tran_amt">Amount</label></th>
        <td><input id="tran_amt" name="tran_amt" value="{{.Body.TranAmt}}" maxlength="12" size="14" required></td>
      </tr>
      <tr>
        <th><label for="merchant_id">Merchant ID</label></th>
        <td><input id="merchant_id" name="merchant_id" value="{{.Body.MerchantID}}" maxlength="9" size="11"></td>
      </tr>
      <tr>
        <th><label for="merchant_name">Merchant Name</label></th>
        <td><input id="merchant_name" name="merchant_name" value="{{.Body.MerchantName}}" maxlength="50" size="30"></td>
      </tr>
      <tr>
        <th><label for="merchant_city">Merchant City</label></th>
        <td><input id="merchant_city" name="merchant_city" value="{{.Body.MerchantCity}}" maxlength="50" size="30"></td>
      </tr>
      <tr>
        <th><label for="merchant_zip">Merchant Zip</label></th>
        <td><input id="merchant_zip" name="merchant_zip" value="{{.Body.MerchantZip}}" maxlength="10" size="12"></td>
      </tr>
    </table>
    <div class="form-actions">
      <button type="submit">Add Transaction</button>
      <a href="/transactions">Cancel</a>
    </div>
  </form>
</section>
{{end}}`

// GetAddTxn serves the COTRN02C add-transaction form (GET).
func (h *Handlers) GetAddTxn(w http.ResponseWriter, r *http.Request) {
	pd := layout.NewPage(r, "Transaction Add").WithBody(addBody{CSRFToken: csrfToken(r)})
	layout.Render(w, pd, addContent)
}

// PostAddTxn processes the COTRN02C add-transaction form (POST).
func (h *Handlers) PostAddTxn(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	body := addBody{
		CardNum:      strings.TrimSpace(r.FormValue("card_num")),
		TranTypeCode: strings.TrimSpace(r.FormValue("tran_type")),
		TranCatCode:  strings.TrimSpace(r.FormValue("tran_cat")),
		TranSource:   strings.TrimSpace(r.FormValue("tran_source")),
		TranDesc:     strings.TrimSpace(r.FormValue("tran_desc")),
		TranAmt:      strings.TrimSpace(r.FormValue("tran_amt")),
		MerchantID:   strings.TrimSpace(r.FormValue("merchant_id")),
		MerchantName: strings.TrimSpace(r.FormValue("merchant_name")),
		MerchantCity: strings.TrimSpace(r.FormValue("merchant_city")),
		MerchantZip:  strings.TrimSpace(r.FormValue("merchant_zip")),
		CSRFToken:    csrfToken(r),
	}

	catCode, err := strconv.ParseInt(body.TranCatCode, 10, 64)
	if err != nil {
		renderFlash(w, r, "Transaction Add", "Invalid category code.", addContent, body)
		return
	}
	amt, err := decimal.NewFromString(body.TranAmt)
	if err != nil {
		renderFlash(w, r, "Transaction Add", "Invalid amount.", addContent, body)
		return
	}
	var merchantID int64
	if body.MerchantID != "" {
		merchantID, err = strconv.ParseInt(body.MerchantID, 10, 64)
		if err != nil {
			renderFlash(w, r, "Transaction Add", "Invalid merchant ID.", addContent, body)
			return
		}
	}

	req := svc.AddRequest{
		TranTypeCode:     body.TranTypeCode,
		TranCatCode:      catCode,
		TranSource:       body.TranSource,
		TranDesc:         body.TranDesc,
		TranAmt:          amt,
		TranMerchantID:   merchantID,
		TranMerchantName: body.MerchantName,
		TranMerchantCity: body.MerchantCity,
		TranMerchantZip:  body.MerchantZip,
		TranCardNum:      body.CardNum,
	}

	rec, err := h.Service.Add(r.Context(), req)
	if err != nil {
		renderFlash(w, r, "Transaction Add", userFriendlyErr(err), addContent, body)
		return
	}

	http.Redirect(w, r, "/transactions/view?id="+rec.TranID, http.StatusSeeOther)
}

// ─── Bill Payment (COBIL00C) ─────────────────────────────────────────────────

type payBody struct {
	CardNum   string
	Amt       string
	CSRFToken string
}

const payContent = `{{define "content"}}
<section class="billpay-screen">
  <form method="post" action="/billing">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <table class="form-table">
      <tr>
        <th><label for="card_num">Card Number</label></th>
        <td><input id="card_num" name="card_num" value="{{.Body.CardNum}}" maxlength="16" size="20" required></td>
      </tr>
      <tr>
        <th><label for="amt">Payment Amount</label></th>
        <td><input id="amt" name="amt" value="{{.Body.Amt}}" maxlength="12" size="14" required></td>
      </tr>
    </table>
    <div class="form-actions">
      <button type="submit">Pay Bill</button>
      <a href="/">Cancel</a>
    </div>
  </form>
</section>
{{end}}`

// GetBillPay serves the COBIL00C bill-payment form (GET).
// Optional query param: card prefills the card number field.
func (h *Handlers) GetBillPay(w http.ResponseWriter, r *http.Request) {
	card := strings.TrimSpace(r.URL.Query().Get("card"))
	pd := layout.NewPage(r, "Bill Payment").WithBody(payBody{CardNum: card, CSRFToken: csrfToken(r)})
	layout.Render(w, pd, payContent)
}

// PostBillPay processes the COBIL00C bill-payment form (POST).
func (h *Handlers) PostBillPay(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	cardNum := strings.TrimSpace(r.FormValue("card_num"))
	amtStr := strings.TrimSpace(r.FormValue("amt"))

	body := payBody{CardNum: cardNum, Amt: amtStr, CSRFToken: csrfToken(r)}

	amt, err := decimal.NewFromString(amtStr)
	if err != nil {
		renderFlash(w, r, "Bill Payment", "Invalid amount.", payContent, body)
		return
	}

	rec, err := h.Service.Pay(r.Context(), svc.PayRequest{CardNum: cardNum, Amt: amt})
	if err != nil {
		renderFlash(w, r, "Bill Payment", userFriendlyErr(err), payContent, body)
		return
	}

	http.Redirect(w, r, "/transactions/view?id="+rec.TranID, http.StatusSeeOther)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func csrfToken(r *http.Request) string {
	pd := layout.NewPage(r, "")
	return pd.CSRFToken
}

func renderFlash(w http.ResponseWriter, r *http.Request, title, msg, tpl string, body any) {
	pd := layout.NewPage(r, title).WithFlash(msg).WithBody(body)
	layout.Render(w, pd, tpl)
}

// userFriendlyErr maps service-layer sentinel errors to BMS-style ERRMSG strings.
func userFriendlyErr(err error) string {
	switch {
	case errors.Is(err, svc.ErrInvalidAmount):
		return "Amount must not be zero."
	case errors.Is(err, svc.ErrAmountNegativeOrZero):
		return "Payment amount must be positive."
	case errors.Is(err, svc.ErrInvalidTranType):
		return "Invalid transaction type code."
	case errors.Is(err, svc.ErrInvalidTranCat):
		return "Invalid transaction category for this type."
	case errors.Is(err, svc.ErrAccountNotFound):
		return "Account not found."
	case errors.Is(err, svc.ErrAccountClosed):
		return "Account is closed."
	case errors.Is(err, svc.ErrCardNotFound):
		return "Card number not found."
	case errors.Is(err, svc.ErrCardInactive):
		return "Card is not active."
	case errors.Is(err, svc.ErrCreditLimitExceeded):
		return "Amount would exceed credit limit."
	case errors.Is(err, svc.ErrPaymentExceedsBalance):
		return "Payment amount exceeds current balance."
	case errors.Is(err, svc.ErrZeroBalance):
		return "No balance owing; nothing to pay."
	default:
		return fmt.Sprintf("Unexpected error: %v", err)
	}
}

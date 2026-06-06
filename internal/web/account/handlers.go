// Package account is the HTTP edge for the account view and update screens.
// It is the Go replacement for:
//
//	COACTVWC  → GET /account/view, POST /account/view   (read-only view)
//	COACTUPC  → GET /account/update, POST /account/update (update with validation)
package account

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/account"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/layout"
)

// Handlers exposes the HTTP entry points for account view/update.
type Handlers struct {
	Service *account.Service
}

// NewHandlers constructs Handlers.
func NewHandlers(svc *account.Service) *Handlers {
	return &Handlers{Service: svc}
}

// ── Account View (COACTVWC) ───────────────────────────────────────────────────

type viewBody struct {
	AcctIDInput  string
	View         *account.View
	SSNFormatted string // pre-formatted as "NNN-NN-NNNN" for display
}

// GetView renders the COACTVW screen.
// With ?acct_id=N it loads and shows the account; without it shows the search form.
func (h *Handlers) GetView(w http.ResponseWriter, r *http.Request) {
	acctIDStr := r.URL.Query().Get("acct_id")
	pd := layout.NewPage(r, "Account View")

	if acctIDStr == "" {
		layout.Render(w, pd.WithBody(viewBody{}), viewContent)
		return
	}

	acctID, err := parseAccountID(acctIDStr)
	if err != nil {
		layout.Render(w, pd.WithFlash(err.Error()).WithBody(viewBody{AcctIDInput: acctIDStr}), viewContent)
		return
	}

	v, err := h.Service.GetByID(r.Context(), acctID)
	if err != nil {
		msg := "Account not found."
		if !errors.Is(err, repo.ErrNotFound) {
			log.Printf("account GetByID error: %v", err)
			msg = "Error loading account. Please try again."
		}
		layout.Render(w, pd.WithFlash(msg).WithBody(viewBody{AcctIDInput: acctIDStr}), viewContent)
		return
	}

	body := viewBody{AcctIDInput: acctIDStr, View: v}
	if v.Customer != nil {
		body.SSNFormatted = formatSSN(v.Customer.CustSSN)
	}
	layout.Render(w, pd.WithBody(body), viewContent)
}

// PostView processes the account search form and redirects to GET with acct_id.
func (h *Handlers) PostView(w http.ResponseWriter, r *http.Request) {
	acctIDStr := strings.TrimSpace(r.FormValue("acct_id"))
	if acctIDStr == "" {
		http.Redirect(w, r, "/account/view", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/account/view?acct_id="+acctIDStr, http.StatusSeeOther)
}

// ── Account Update (COACTUPC) ─────────────────────────────────────────────────

type updateBody struct {
	AcctIDInput string
	View        *account.View // nil = search phase; non-nil = edit phase
	Form        *updateForm   // mirrors submitted form values for re-render on error
}

// updateForm holds form field values for re-rendering after a validation error.
type updateForm struct {
	ActiveStatus    string
	CreditLimit     string
	CashCreditLimit string
	CurrBal         string
	CurrCycCredit   string
	CurrCycDebit    string
	OpenDate        string
	ExpirationDate  string
	ReissueDate     string
	GroupID         string
	FirstName       string
	MiddleName      string
	LastName        string
	AddrLine1       string
	AddrLine2       string
	City            string
	StateCode       string
	Zip             string
	Country         string
	Phone1Area      string
	Phone1Exchange  string
	Phone1Line      string
	Phone2Area      string
	Phone2Exchange  string
	Phone2Line      string
	SSNArea         string
	SSNGroup        string
	SSNSerial       string
	GovtIssuedID    string
	DOB             string
	EFTAccountID    string
	PriCardHolder   string
	FICOScore       string
}

// GetUpdate renders the COACTUPC screen.
// With ?acct_id=N it loads current data into the edit form; without it shows the search form.
func (h *Handlers) GetUpdate(w http.ResponseWriter, r *http.Request) {
	acctIDStr := r.URL.Query().Get("acct_id")
	pd := layout.NewPage(r, "Account Update")

	if acctIDStr == "" {
		layout.Render(w, pd.WithBody(updateBody{}), updateContent)
		return
	}

	acctID, err := parseAccountID(acctIDStr)
	if err != nil {
		layout.Render(w, pd.WithFlash(err.Error()).WithBody(updateBody{AcctIDInput: acctIDStr}), updateContent)
		return
	}

	v, err := h.Service.GetByID(r.Context(), acctID)
	if err != nil {
		msg := "Account not found."
		if !errors.Is(err, repo.ErrNotFound) {
			log.Printf("account GetByID error: %v", err)
			msg = "Error loading account. Please try again."
		}
		layout.Render(w, pd.WithFlash(msg).WithBody(updateBody{AcctIDInput: acctIDStr}), updateContent)
		return
	}

	layout.Render(w, pd.WithBody(updateBody{AcctIDInput: acctIDStr, View: v, Form: viewToForm(v)}), updateContent)
}

// PostUpdate processes either a search (action=search) or a save (action=update).
func (h *Handlers) PostUpdate(w http.ResponseWriter, r *http.Request) {
	action := r.FormValue("action")

	if action == "search" {
		acctIDStr := strings.TrimSpace(r.FormValue("acct_id"))
		if acctIDStr == "" {
			http.Redirect(w, r, "/account/update", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/account/update?acct_id="+acctIDStr, http.StatusSeeOther)
		return
	}

	// action == "update"
	acctIDStr := strings.TrimSpace(r.FormValue("acct_id"))
	acctID, err := parseAccountID(acctIDStr)
	if err != nil {
		pd := layout.NewPage(r, "Account Update")
		layout.Render(w, pd.WithFlash(err.Error()).WithBody(updateBody{AcctIDInput: acctIDStr}), updateContent)
		return
	}

	form := formFromRequest(r)
	req, parseErr := buildUpdateRequest(acctID, r)

	// Re-read view data for re-render (needed even on error).
	v, loadErr := h.Service.GetByID(r.Context(), acctID)
	pd := layout.NewPage(r, "Account Update")

	if loadErr != nil {
		layout.Render(w, pd.WithFlash("Could not reload account.").WithBody(updateBody{AcctIDInput: acctIDStr}), updateContent)
		return
	}

	if parseErr != nil {
		layout.Render(w, pd.WithFlash(parseErr.Error()).WithBody(updateBody{AcctIDInput: acctIDStr, View: v, Form: form}), updateContent)
		return
	}

	if err := h.Service.Update(r.Context(), req); err != nil {
		msg := err.Error()
		if !account.IsValidationError(err) {
			log.Printf("account Update error: %v", err)
			msg = "Changes unsuccessful. Please try again."
		}
		layout.Render(w, pd.WithFlash(msg).WithBody(updateBody{AcctIDInput: acctIDStr, View: v, Form: form}), updateContent)
		return
	}

	// Success: redirect to view.
	http.Redirect(w, r, fmt.Sprintf("/account/view?acct_id=%d", acctID), http.StatusSeeOther)
}

// parseAccountID validates and converts an account-ID string.
func parseAccountID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("Account number not provided")
	}
	if !isNumeric(s) {
		return 0, fmt.Errorf("Account Filter must be a non-zero 11 digit number")
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("Account Filter must be a non-zero 11 digit number")
	}
	return n, nil
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// formFromRequest extracts all form fields into an updateForm for re-render.
func formFromRequest(r *http.Request) *updateForm {
	return &updateForm{
		ActiveStatus:    r.FormValue("active_status"),
		CreditLimit:     r.FormValue("credit_limit"),
		CashCreditLimit: r.FormValue("cash_credit_limit"),
		CurrBal:         r.FormValue("curr_bal"),
		CurrCycCredit:   r.FormValue("curr_cyc_credit"),
		CurrCycDebit:    r.FormValue("curr_cyc_debit"),
		OpenDate:        r.FormValue("open_date"),
		ExpirationDate:  r.FormValue("expiration_date"),
		ReissueDate:     r.FormValue("reissue_date"),
		GroupID:         r.FormValue("group_id"),
		FirstName:       r.FormValue("first_name"),
		MiddleName:      r.FormValue("middle_name"),
		LastName:        r.FormValue("last_name"),
		AddrLine1:       r.FormValue("addr_line1"),
		AddrLine2:       r.FormValue("addr_line2"),
		City:            r.FormValue("city"),
		StateCode:       r.FormValue("state_code"),
		Zip:             r.FormValue("zip"),
		Country:         r.FormValue("country"),
		Phone1Area:      r.FormValue("phone1_area"),
		Phone1Exchange:  r.FormValue("phone1_exchange"),
		Phone1Line:      r.FormValue("phone1_line"),
		Phone2Area:      r.FormValue("phone2_area"),
		Phone2Exchange:  r.FormValue("phone2_exchange"),
		Phone2Line:      r.FormValue("phone2_line"),
		SSNArea:         r.FormValue("ssn_area"),
		SSNGroup:        r.FormValue("ssn_group"),
		SSNSerial:       r.FormValue("ssn_serial"),
		GovtIssuedID:    r.FormValue("govt_issued_id"),
		DOB:             r.FormValue("dob"),
		EFTAccountID:    r.FormValue("eft_account_id"),
		PriCardHolder:   r.FormValue("pri_card_holder"),
		FICOScore:       r.FormValue("fico_score"),
	}
}

// buildUpdateRequest parses form values into an account.UpdateRequest.
// Returns a parse error for values that cannot be converted to the required types.
func buildUpdateRequest(acctID int64, r *http.Request) (account.UpdateRequest, error) {
	var req account.UpdateRequest
	req.AcctID = acctID

	req.ActiveStatus = r.FormValue("active_status")
	req.OpenDate = r.FormValue("open_date")
	req.ExpirationDate = r.FormValue("expiration_date")
	req.ReissueDate = r.FormValue("reissue_date")
	req.GroupID = r.FormValue("group_id")

	var parseErr error
	parseDecimal := func(field string) decimal.Decimal {
		s := strings.TrimSpace(r.FormValue(field))
		d, err := decimal.NewFromString(s)
		if err != nil && parseErr == nil {
			parseErr = fmt.Errorf("%s: invalid number %q", field, s)
		}
		return d
	}

	req.CreditLimit = parseDecimal("credit_limit")
	req.CashCreditLimit = parseDecimal("cash_credit_limit")
	req.CurrBal = parseDecimal("curr_bal")
	req.CurrCycCredit = parseDecimal("curr_cyc_credit")
	req.CurrCycDebit = parseDecimal("curr_cyc_debit")

	if parseErr != nil {
		return req, parseErr
	}

	// SSN: three separate fields assembled into 9-digit int64.
	// Enforce exact widths before assembly to prevent corruption of the packed value.
	ssnArea := strings.TrimSpace(r.FormValue("ssn_area"))
	ssnGroup := strings.TrimSpace(r.FormValue("ssn_group"))
	ssnSerial := strings.TrimSpace(r.FormValue("ssn_serial"))
	if len(ssnArea) != 3 || !isNumeric(ssnArea) {
		return req, fmt.Errorf("SSN area must be exactly 3 digits")
	}
	if len(ssnGroup) != 2 || !isNumeric(ssnGroup) {
		return req, fmt.Errorf("SSN group must be exactly 2 digits")
	}
	if len(ssnSerial) != 4 || !isNumeric(ssnSerial) {
		return req, fmt.Errorf("SSN serial must be exactly 4 digits")
	}
	area, _ := strconv.ParseInt(ssnArea, 10, 64)
	grp, _ := strconv.ParseInt(ssnGroup, 10, 64)
	ser, _ := strconv.ParseInt(ssnSerial, 10, 64)
	req.SSN = area*1_000_000 + grp*10_000 + ser

	req.FirstName = r.FormValue("first_name")
	req.MiddleName = r.FormValue("middle_name")
	req.LastName = r.FormValue("last_name")
	req.AddrLine1 = r.FormValue("addr_line1")
	req.AddrLine2 = r.FormValue("addr_line2")
	req.AddrLine3 = r.FormValue("city")
	req.AddrStateCode = r.FormValue("state_code")
	req.AddrZip = r.FormValue("zip")
	req.AddrCountryCode = r.FormValue("country")
	req.GovtIssuedID = r.FormValue("govt_issued_id")
	req.DOB = r.FormValue("dob")
	req.EFTAccountID = r.FormValue("eft_account_id")
	req.PriCardHolderInd = r.FormValue("pri_card_holder")

	ficoStr := strings.TrimSpace(r.FormValue("fico_score"))
	fico, err := strconv.ParseInt(ficoStr, 10, 64)
	if err != nil {
		return req, fmt.Errorf("FICO score must be a number")
	}
	req.FICOCreditScore = fico

	// Phone: three-part fields assembled into (NNN)NNN-NNNN, empty if all parts empty.
	req.PhoneNum1 = assemblePhone(r.FormValue("phone1_area"), r.FormValue("phone1_exchange"), r.FormValue("phone1_line"))
	req.PhoneNum2 = assemblePhone(r.FormValue("phone2_area"), r.FormValue("phone2_exchange"), r.FormValue("phone2_line"))

	return req, nil
}

// assemblePhone builds "(NNN)NNN-NNNN" from three parts, or returns "" if all are blank.
func assemblePhone(area, exchange, line string) string {
	area = strings.TrimSpace(area)
	exchange = strings.TrimSpace(exchange)
	line = strings.TrimSpace(line)
	if area == "" && exchange == "" && line == "" {
		return ""
	}
	return fmt.Sprintf("(%s)%s-%s", area, exchange, line)
}

// viewToForm converts the current account view data into pre-filled form values.
func viewToForm(v *account.View) *updateForm {
	if v == nil {
		return &updateForm{}
	}
	f := &updateForm{}
	if v.Account != nil {
		f.ActiveStatus = v.Account.AcctActiveStatus
		f.CreditLimit = v.Account.AcctCreditLimit.String()
		f.CashCreditLimit = v.Account.AcctCashCreditLimit.String()
		f.CurrBal = v.Account.AcctCurrBal.String()
		f.CurrCycCredit = v.Account.AcctCurrCycCredit.String()
		f.CurrCycDebit = v.Account.AcctCurrCycDebit.String()
		f.OpenDate = v.Account.AcctOpenDate
		f.ExpirationDate = v.Account.AcctExpirationDate
		f.ReissueDate = v.Account.AcctReissueDate
		f.GroupID = v.Account.AcctGroupID
	}
	if v.Customer != nil {
		c := v.Customer
		f.FirstName = c.CustFirstName
		f.MiddleName = c.CustMiddleName
		f.LastName = c.CustLastName
		f.AddrLine1 = c.CustAddrLine1
		f.AddrLine2 = c.CustAddrLine2
		f.City = c.CustAddrLine3
		f.StateCode = c.CustAddrStateCode
		f.Zip = c.CustAddrZip
		f.Country = c.CustAddrCountryCode
		f.GovtIssuedID = c.CustGovtIssuedID
		f.DOB = c.CustDOB
		f.EFTAccountID = c.CustEFTAccountID
		f.PriCardHolder = c.CustPriCardHolderInd
		f.FICOScore = strconv.FormatInt(c.CustFICOCreditScore, 10)

		// Split SSN (9-digit int64) into three display parts.
		ssn := c.CustSSN
		f.SSNArea = fmt.Sprintf("%03d", ssn/1_000_000)
		f.SSNGroup = fmt.Sprintf("%02d", (ssn/10_000)%100)
		f.SSNSerial = fmt.Sprintf("%04d", ssn%10_000)

		// Split phone "(NNN)NNN-NNNN" into three parts.
		f.Phone1Area, f.Phone1Exchange, f.Phone1Line = splitPhone(c.CustPhoneNum1)
		f.Phone2Area, f.Phone2Exchange, f.Phone2Line = splitPhone(c.CustPhoneNum2)
	}
	return f
}

// formatSSN formats a 9-digit int64 as "NNN-NN-NNNN".
func formatSSN(ssn int64) string {
	area := ssn / 1_000_000
	group := (ssn / 10_000) % 100
	serial := ssn % 10_000
	return fmt.Sprintf("%03d-%02d-%04d", area, group, serial)
}

// splitPhone parses "(NNN)NNN-NNNN" into three parts; returns empty strings on mismatch.
func splitPhone(phone string) (area, exchange, line string) {
	phone = strings.TrimSpace(phone)
	if len(phone) == 13 && phone[0] == '(' && phone[4] == ')' && phone[8] == '-' {
		return phone[1:4], phone[5:8], phone[9:13]
	}
	return "", "", ""
}

// ── HTML templates ────────────────────────────────────────────────────────────

const viewContent = `{{define "content"}}
<section class="account-screen">
  <h2 class="screen-title">Account View (COACTVWC)</h2>

  <form method="post" action="/account/view" class="search-form">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <div class="form-row">
      <label for="acct_id">Account Number:</label>
      <input type="text" id="acct_id" name="acct_id" maxlength="11"
             value="{{.Body.AcctIDInput}}" placeholder="11-digit account number">
      <button type="submit" class="btn-primary">Search (Enter)</button>
    </div>
  </form>

  {{with .Body.View}}
  <div class="account-detail">
    <h3>Account Information</h3>
    <table class="detail-table">
      <tr><th>Account ID</th><td>{{.Account.AcctID}}</td>
          <th>Status</th><td>{{.Account.AcctActiveStatus}}</td></tr>
      <tr><th>Open Date</th><td>{{.Account.AcctOpenDate}}</td>
          <th>Credit Limit</th><td>{{.Account.AcctCreditLimit}}</td></tr>
      <tr><th>Expiry Date</th><td>{{.Account.AcctExpirationDate}}</td>
          <th>Cash Credit Limit</th><td>{{.Account.AcctCashCreditLimit}}</td></tr>
      <tr><th>Reissue Date</th><td>{{.Account.AcctReissueDate}}</td>
          <th>Current Balance</th><td>{{.Account.AcctCurrBal}}</td></tr>
      <tr><th>Group ID</th><td>{{.Account.AcctGroupID}}</td>
          <th>Cycle Credit</th><td>{{.Account.AcctCurrCycCredit}}</td></tr>
      <tr><th>Card Number</th><td>{{.CardNum}}</td>
          <th>Cycle Debit</th><td>{{.Account.AcctCurrCycDebit}}</td></tr>
    </table>

    {{with .Customer}}
    <h3>Customer Information</h3>
    <table class="detail-table">
      <tr><th>Customer ID</th><td>{{.CustID}}</td>
          <th>SSN</th><td>{{$.Body.SSNFormatted}}</td></tr>
      <tr><th>Name</th><td colspan="3">{{.CustFirstName}} {{.CustMiddleName}} {{.CustLastName}}</td></tr>
      <tr><th>Address</th><td colspan="3">{{.CustAddrLine1}}</td></tr>
      {{if .CustAddrLine2}}<tr><td></td><td colspan="3">{{.CustAddrLine2}}</td></tr>{{end}}
      <tr><td></td><td colspan="3">{{.CustAddrLine3}}, {{.CustAddrStateCode}} {{.CustAddrZip}} {{.CustAddrCountryCode}}</td></tr>
      <tr><th>DOB</th><td>{{.CustDOB}}</td>
          <th>FICO</th><td>{{.CustFICOCreditScore}}</td></tr>
      <tr><th>Phone 1</th><td>{{.CustPhoneNum1}}</td>
          <th>Phone 2</th><td>{{.CustPhoneNum2}}</td></tr>
      <tr><th>Govt ID</th><td>{{.CustGovtIssuedID}}</td>
          <th>EFT Account</th><td>{{.CustEFTAccountID}}</td></tr>
      <tr><th>Primary Card Holder</th><td>{{.CustPriCardHolderInd}}</td></tr>
    </table>
    {{end}}

    <div class="action-row">
      <a href="/account/update?acct_id={{.Account.AcctID}}" class="btn-secondary">Edit (Update)</a>
      <a href="/account/view" class="btn-secondary">New Search</a>
    </div>
  </div>
  {{end}}
</section>
{{end}}`

const updateContent = `{{define "content"}}
<section class="account-screen">
  <h2 class="screen-title">Account Update (COACTUPC)</h2>

  {{if .Body.View}}
  <form method="post" action="/account/update" class="edit-form">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <input type="hidden" name="action" value="update">
    <input type="hidden" name="acct_id" value="{{.Body.AcctIDInput}}">

    <fieldset>
      <legend>Account Fields</legend>
      <div class="form-grid">
        <label>Account ID</label><span class="readonly">{{.Body.View.Account.AcctID}}</span>
        <label>Status (Y/N)</label>
        <input type="text" name="active_status" maxlength="1" value="{{.Body.Form.ActiveStatus}}" required>

        <label>Open Date (YYYY-MM-DD)</label>
        <input type="text" name="open_date" maxlength="10" value="{{.Body.Form.OpenDate}}" placeholder="YYYY-MM-DD">
        <label>Credit Limit</label>
        <input type="text" name="credit_limit" value="{{.Body.Form.CreditLimit}}">

        <label>Expiry Date (YYYY-MM-DD)</label>
        <input type="text" name="expiration_date" maxlength="10" value="{{.Body.Form.ExpirationDate}}" placeholder="YYYY-MM-DD">
        <label>Cash Credit Limit</label>
        <input type="text" name="cash_credit_limit" value="{{.Body.Form.CashCreditLimit}}">

        <label>Reissue Date (YYYY-MM-DD)</label>
        <input type="text" name="reissue_date" maxlength="10" value="{{.Body.Form.ReissueDate}}" placeholder="YYYY-MM-DD">
        <label>Current Balance</label>
        <input type="text" name="curr_bal" value="{{.Body.Form.CurrBal}}">

        <label>Group ID</label>
        <input type="text" name="group_id" maxlength="10" value="{{.Body.Form.GroupID}}">
        <label>Cycle Credit</label>
        <input type="text" name="curr_cyc_credit" value="{{.Body.Form.CurrCycCredit}}">

        <label>Card Number</label><span class="readonly">{{.Body.View.CardNum}}</span>
        <label>Cycle Debit</label>
        <input type="text" name="curr_cyc_debit" value="{{.Body.Form.CurrCycDebit}}">
      </div>
    </fieldset>

    <fieldset>
      <legend>Customer Fields</legend>
      <div class="form-grid">
        {{with .Body.View.Customer}}
        <label>Customer ID</label><span class="readonly">{{.CustID}}</span>
        {{end}}
        <label>SSN (Area / Group / Serial)</label>
        <div class="ssn-group">
          <input type="text" name="ssn_area" maxlength="3" size="3" value="{{.Body.Form.SSNArea}}" placeholder="NNN">-
          <input type="text" name="ssn_group" maxlength="2" size="2" value="{{.Body.Form.SSNGroup}}" placeholder="NN">-
          <input type="text" name="ssn_serial" maxlength="4" size="4" value="{{.Body.Form.SSNSerial}}" placeholder="NNNN">
        </div>

        <label>DOB (YYYY-MM-DD)</label>
        <input type="text" name="dob" maxlength="10" value="{{.Body.Form.DOB}}" placeholder="YYYY-MM-DD">
        <label>FICO Score (300-850)</label>
        <input type="text" name="fico_score" maxlength="3" value="{{.Body.Form.FICOScore}}">

        <label>First Name</label>
        <input type="text" name="first_name" maxlength="25" value="{{.Body.Form.FirstName}}">
        <label>Middle Name</label>
        <input type="text" name="middle_name" maxlength="25" value="{{.Body.Form.MiddleName}}">

        <label>Last Name</label>
        <input type="text" name="last_name" maxlength="25" value="{{.Body.Form.LastName}}">

        <label>Address Line 1</label>
        <input type="text" name="addr_line1" maxlength="50" value="{{.Body.Form.AddrLine1}}">
        <label>Address Line 2</label>
        <input type="text" name="addr_line2" maxlength="50" value="{{.Body.Form.AddrLine2}}">

        <label>City</label>
        <input type="text" name="city" maxlength="50" value="{{.Body.Form.City}}">
        <label>State (2-letter)</label>
        <input type="text" name="state_code" maxlength="2" size="2" value="{{.Body.Form.StateCode}}">

        <label>ZIP (5-digit)</label>
        <input type="text" name="zip" maxlength="10" size="5" value="{{.Body.Form.Zip}}">
        <label>Country (3-letter)</label>
        <input type="text" name="country" maxlength="3" size="3" value="{{.Body.Form.Country}}">

        <label>Phone 1</label>
        <div class="phone-group">
          (<input type="text" name="phone1_area" maxlength="3" size="3" value="{{.Body.Form.Phone1Area}}">)
          <input type="text" name="phone1_exchange" maxlength="3" size="3" value="{{.Body.Form.Phone1Exchange}}">-
          <input type="text" name="phone1_line" maxlength="4" size="4" value="{{.Body.Form.Phone1Line}}">
        </div>
        <label>Govt Issued ID</label>
        <input type="text" name="govt_issued_id" maxlength="20" value="{{.Body.Form.GovtIssuedID}}">

        <label>Phone 2</label>
        <div class="phone-group">
          (<input type="text" name="phone2_area" maxlength="3" size="3" value="{{.Body.Form.Phone2Area}}">)
          <input type="text" name="phone2_exchange" maxlength="3" size="3" value="{{.Body.Form.Phone2Exchange}}">-
          <input type="text" name="phone2_line" maxlength="4" size="4" value="{{.Body.Form.Phone2Line}}">
        </div>
        <label>EFT Account ID</label>
        <input type="text" name="eft_account_id" maxlength="10" value="{{.Body.Form.EFTAccountID}}">

        <label>Primary Card Holder (Y/N)</label>
        <input type="text" name="pri_card_holder" maxlength="1" size="1" value="{{.Body.Form.PriCardHolder}}">
      </div>
    </fieldset>

    <div class="action-row">
      <button type="submit" class="btn-primary">Save (F5)</button>
      <a href="/account/view?acct_id={{.Body.AcctIDInput}}" class="btn-secondary">Cancel (F12)</a>
    </div>
  </form>
  {{else}}
  <form method="post" action="/account/update" class="search-form">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <input type="hidden" name="action" value="search">
    <div class="form-row">
      <label for="acct_id">Account Number:</label>
      <input type="text" id="acct_id" name="acct_id" maxlength="11"
             value="{{.Body.AcctIDInput}}" placeholder="11-digit account number">
      <button type="submit" class="btn-primary">Search (Enter)</button>
    </div>
  </form>
  {{end}}
</section>
{{end}}`

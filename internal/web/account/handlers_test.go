package account_test

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	authpkg "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/auth"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/audit"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/domain"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	svcaccount "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/account"
	webaccount "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/account"
	webauth "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/auth"
)

// testEnv holds a running test HTTP server with session + CSRF middleware,
// a seeded SQLite DB, and an authenticated cookie jar.
type testEnv struct {
	srv      *httptest.Server
	client   *http.Client
	store    *authpkg.MemorySessionStore
	users    *repo.InMemoryUserSec
	acctSvc  *svcaccount.Service
	acctRepo *sqlite.AccountStore
	custRepo *sqlite.CustomerStore
	xrefRepo *sqlite.CardXrefStore
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	acctRepo := sqlite.NewAccountStore(db)
	custRepo := sqlite.NewCustomerStore(db)
	xrefRepo := sqlite.NewCardXrefStore(db)

	users := repo.NewInMemoryUserSec()
	store := authpkg.NewMemorySessionStore()
	sink := audit.NewMemorySink()
	authSvc := authpkg.NewService(users, store, sink)

	if err := authpkg.SeedDefaultUsers(context.Background(), users, 4); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	acctSvc := svcaccount.New(acctRepo, custRepo, xrefRepo)
	accountHandlers := webaccount.NewHandlers(acctSvc)

	// Use the real webauth.Handlers (same pattern as the auth package's own tests).
	authH := webauth.NewHandlers(authSvc)
	authH.CookieOptions.Secure = false // httptest is plain HTTP

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", authH.GetLogin)
	mux.HandleFunc("POST /login", authH.PostLogin)

	// Account routes under RequireUser.
	accountMux := http.NewServeMux()
	accountMux.HandleFunc("GET /account/view", accountHandlers.GetView)
	accountMux.HandleFunc("POST /account/view", accountHandlers.PostView)
	accountMux.HandleFunc("GET /account/update", accountHandlers.GetUpdate)
	accountMux.HandleFunc("POST /account/update", accountHandlers.PostUpdate)
	mux.Handle("/account/", authpkg.RequireUser("/login")(accountMux))

	handler := authpkg.LoadSession(store)(authpkg.CSRFProtect()(mux))
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:           jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}

	return &testEnv{
		srv:      srv,
		client:   client,
		store:    store,
		users:    users,
		acctSvc:  acctSvc,
		acctRepo: acctRepo,
		custRepo: custRepo,
		xrefRepo: xrefRepo,
	}
}

func (env *testEnv) loginAs(t *testing.T, userID, password string) {
	t.Helper()
	// GET /login to grab CSRF cookie.
	resp, err := env.client.Get(env.srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	resp.Body.Close()

	// Extract CSRF token from cookie jar.
	u, _ := url.Parse(env.srv.URL)
	var csrfToken string
	for _, c := range env.client.Jar.Cookies(u) {
		if c.Name == authpkg.CSRFCookieName {
			csrfToken = c.Value
		}
	}

	if csrfToken == "" {
		t.Fatalf("loginAs: pre-login CSRF cookie not found in jar (checked %q)", authpkg.CSRFCookieName)
	}

	form := url.Values{
		"csrf_token": {csrfToken},
		"user_id":    {userID},
		"password":   {password},
	}
	resp, err = env.client.PostForm(env.srv.URL+"/login", form)
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("loginAs: POST /login: want 303, got %d; body=%q", resp.StatusCode, body)
	}
}

func (env *testEnv) seedAccount(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	acct := &domain.AccountRecord{
		AcctID:              70000001001,
		AcctActiveStatus:    "Y",
		AcctCurrBal:         decimal.RequireFromString("100.00"),
		AcctCreditLimit:     decimal.RequireFromString("5000.00"),
		AcctCashCreditLimit: decimal.RequireFromString("1000.00"),
		AcctOpenDate:        "2020-01-15",
		AcctExpirationDate:  "2025-12-31",
		AcctReissueDate:     "2023-01-15",
		AcctCurrCycCredit:   decimal.Zero,
		AcctCurrCycDebit:    decimal.Zero,
		AcctAddrZip:         "90210",
		AcctGroupID:         "GRP001",
	}
	cust := &domain.CustomerRecord{
		CustID:               1001,
		CustFirstName:        "John",
		CustMiddleName:       "A",
		CustLastName:         "Doe",
		CustAddrLine1:        "123 Main St",
		CustAddrLine2:        "Apt 4",
		CustAddrLine3:        "Beverly Hills",
		CustAddrStateCode:    "CA",
		CustAddrCountryCode:  "USA",
		CustAddrZip:          "90210",
		CustPhoneNum1:        "(310)555-1234",
		CustPhoneNum2:        "",
		CustSSN:              123456789,
		CustGovtIssuedID:     "DL12345678",
		CustDOB:              "1980-06-15",
		CustEFTAccountID:     "1234567890",
		CustPriCardHolderInd: "Y",
		CustFICOCreditScore:  750,
	}
	xref := &domain.CardXrefRecord{
		XrefCardNum: "4111111111111111",
		XrefCustID:  1001,
		XrefAcctID:  70000001001,
	}

	if err := env.acctRepo.Create(ctx, acct); err != nil {
		t.Fatalf("seed account: %v", err)
	}
	if err := env.custRepo.Create(ctx, cust); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	if err := env.xrefRepo.Create(ctx, xref); err != nil {
		t.Fatalf("seed xref: %v", err)
	}
}

// csrfToken extracts the CSRF token from the session cookie jar.
// The CSRF cookie name is "carddemo_csrf" (authpkg.CSRFCookieName).
func (env *testEnv) csrfToken(t *testing.T) string {
	t.Helper()
	u, _ := url.Parse(env.srv.URL)
	for _, c := range env.client.Jar.Cookies(u) {
		if c.Name == authpkg.CSRFCookieName {
			return c.Value
		}
	}
	return ""
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestGetView_EmptySearchForm(t *testing.T) {
	env := newTestEnv(t)
	env.loginAs(t, "USER0001", "PASSWORD")

	resp, err := env.client.Get(env.srv.URL + "/account/view")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetView_AccountFound(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount(t)
	env.loginAs(t, "USER0001", "PASSWORD")

	resp, err := env.client.Get(env.srv.URL + "/account/view?acct_id=70000001001")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "70000001001") {
		t.Errorf("expected account ID in response body")
	}
	if !strings.Contains(body, "John") {
		t.Errorf("expected customer name in response body")
	}
}

func TestGetView_AccountNotFound(t *testing.T) {
	env := newTestEnv(t)
	env.loginAs(t, "USER0001", "PASSWORD")

	resp, err := env.client.Get(env.srv.URL + "/account/view?acct_id=99999999999")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with error flash, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "not found") && !strings.Contains(body, "Account") {
		t.Errorf("expected not-found message, got body: %s", body[:min(len(body), 200)])
	}
}

func TestPostView_RedirectsToGet(t *testing.T) {
	env := newTestEnv(t)
	env.loginAs(t, "USER0001", "PASSWORD")

	// Need CSRF token — first fetch the view page to populate session.
	resp, _ := env.client.Get(env.srv.URL + "/account/view")
	resp.Body.Close()

	form := url.Values{
		"csrf_token": {env.csrfToken(t)},
		"acct_id":    {"70000001001"},
	}
	resp, err := env.client.PostForm(env.srv.URL+"/account/view", form)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "70000001001") {
		t.Errorf("redirect location should contain acct_id, got %q", loc)
	}
}

func TestGetUpdate_EmptySearchForm(t *testing.T) {
	env := newTestEnv(t)
	env.loginAs(t, "USER0001", "PASSWORD")

	resp, err := env.client.Get(env.srv.URL + "/account/update")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetUpdate_AccountPreFilled(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount(t)
	env.loginAs(t, "USER0001", "PASSWORD")

	resp, err := env.client.Get(env.srv.URL + "/account/update?acct_id=70000001001")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "value=\"update\"") {
		t.Errorf("expected edit form with action=update in response, got body[:200]=%q", body[:min(len(body), 200)])
	}
}

func TestPostUpdate_ValidationError(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount(t)
	env.loginAs(t, "USER0001", "PASSWORD")

	// Fetch update page to populate CSRF.
	resp, _ := env.client.Get(env.srv.URL + "/account/update?acct_id=70000001001")
	resp.Body.Close()

	form := validUpdateForm(env.csrfToken(t), "70000001001")
	form.Set("active_status", "X") // invalid: not Y or N
	resp, err := env.client.PostForm(env.srv.URL+"/account/update", form)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (re-render with error), got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "Account Status") {
		t.Errorf("expected validation error in response, got body[:300]=%q", body[:min(len(body), 300)])
	}
}

func TestPostUpdate_SuccessRedirectsToView(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount(t)
	env.loginAs(t, "USER0001", "PASSWORD")

	resp, _ := env.client.Get(env.srv.URL + "/account/update?acct_id=70000001001")
	resp.Body.Close()

	form := validUpdateForm(env.csrfToken(t), "70000001001")
	resp, err := env.client.PostForm(env.srv.URL+"/account/update", form)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		body := readBody(t, resp)
		t.Errorf("expected 303, got %d; body[:300]=%q", resp.StatusCode, body[:min(len(body), 300)])
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/account/view") {
		t.Errorf("expected redirect to account view, got %q", loc)
	}
}

func TestGetView_UnauthenticatedRedirects(t *testing.T) {
	env := newTestEnv(t)
	// No login.
	jar, _ := cookiejar.New(nil)
	anonClient := &http.Client{
		Jar:           jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := anonClient.Get(env.srv.URL + "/account/view")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for unauthenticated, got %d", resp.StatusCode)
	}
}

// validUpdateForm builds a complete valid update form for account 70000001001 / customer 1001.
func validUpdateForm(csrfToken, acctID string) url.Values {
	return url.Values{
		"csrf_token":        {csrfToken},
		"action":            {"update"},
		"acct_id":           {acctID},
		"active_status":     {"Y"},
		"open_date":         {"2020-01-15"},
		"expiration_date":   {"2025-12-31"},
		"reissue_date":      {"2023-01-15"},
		"credit_limit":      {"5000.00"},
		"cash_credit_limit": {"1000.00"},
		"curr_bal":          {"100.00"},
		"curr_cyc_credit":   {"0"},
		"curr_cyc_debit":    {"0"},
		"group_id":          {"GRP001"},
		"first_name":        {"John"},
		"middle_name":       {"A"},
		"last_name":         {"Doe"},
		"addr_line1":        {"123 Main St"},
		"addr_line2":        {"Apt 4"},
		"city":              {"Beverly Hills"},
		"state_code":        {"CA"},
		"zip":               {"90210"},
		"country":           {"USA"},
		"phone1_area":       {"310"},
		"phone1_exchange":   {"555"},
		"phone1_line":       {"1234"},
		"phone2_area":       {""},
		"phone2_exchange":   {""},
		"phone2_line":       {""},
		"ssn_area":          {"123"},
		"ssn_group":         {"45"},
		"ssn_serial":        {"6789"},
		"govt_issued_id":    {"DL12345678"},
		"dob":               {"1980-06-15"},
		"eft_account_id":    {"1234567890"},
		"pri_card_holder":   {"Y"},
		"fico_score":        {"750"},
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

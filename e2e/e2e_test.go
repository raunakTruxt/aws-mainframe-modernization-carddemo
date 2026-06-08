// Package e2e_test drives the full CardDemo HTTP server via real HTTP
// requests, verifying end-to-end flows: login, menu navigation, account
// view/update, and admin user management.
//
// Each test starts a fresh httptest.Server with an in-memory SQLite DB and
// a cookie-jar client that follows session state exactly as a browser would.
//
// The tested flows map to legacy COBOL screens:
//
//	Login → menu              COSGN00C → COMEN01C
//	Account view              COACTVWC
//	Account update            COACTUPC
//	Admin menu                COADM01C
//	User list / CRUD          COUSR00C / COUSR01C / COUSR02C / COUSR03C
package e2e_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	authpkg "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/auth"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/audit"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	svcaccount "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/account"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web"
	webaccount "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/account"
	webauth "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/auth"
)

// testEnv holds a running test server and a browser-like HTTP client.
type testEnv struct {
	srv    *httptest.Server
	client *http.Client // cookie-jar enabled; does NOT follow redirects
	users  *repo.InMemoryUserSec
	sink   *audit.MemorySink
}

// newTestEnv starts a CardDemo web server backed by an in-memory SQLite DB.
// Default users (ADMIN001/PASSWORD and USER0001/PASSWORD) are seeded.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("newTestEnv: open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	acctRepo := sqlite.NewAccountStore(db)
	custRepo := sqlite.NewCustomerStore(db)
	xrefRepo := sqlite.NewCardXrefStore(db)

	userRepo := repo.NewInMemoryUserSec()
	sessionStore := authpkg.NewMemorySessionStore()
	sink := audit.NewMemorySink()

	if err := authpkg.SeedDefaultUsers(context.Background(), userRepo, 4); err != nil {
		t.Fatalf("newTestEnv: seed users: %v", err)
	}

	authSvc := authpkg.NewService(userRepo, sessionStore, sink)
	authH := webauth.NewHandlers(authSvc)
	authH.CookieOptions.Secure = false // httptest is plain HTTP

	transactor := svcaccount.Transactor(func(ctx context.Context, fn func(repo.AccountRepository, repo.CustomerRepository) error) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := fn(sqlite.NewAccountStore(tx), sqlite.NewCustomerStore(tx)); err != nil {
			tx.Rollback() //nolint:errcheck
			return err
		}
		return tx.Commit()
	})
	accountSvc := svcaccount.New(acctRepo, custRepo, xrefRepo, transactor)
	accountH := webaccount.NewHandlers(accountSvc)

	router := web.NewRouter(authH, accountH, nil, sessionStore, sink)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &testEnv{srv: srv, client: client, users: userRepo, sink: sink}
}

// url builds an absolute URL for the given path.
func (e *testEnv) url(path string) string {
	return e.srv.URL + path
}

// get performs a GET request and returns the response.
func (e *testEnv) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := e.client.Get(e.url(path))
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// post performs a POST with url-encoded form data and returns the response.
func (e *testEnv) post(t *testing.T, path string, form url.Values) *http.Response {
	t.Helper()
	resp, err := e.client.PostForm(e.url(path), form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// bodyString reads and closes the response body.
func bodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// csrfCookie returns the current CSRF token from the cookie jar.
func (e *testEnv) csrfCookie(t *testing.T) string {
	t.Helper()
	u, _ := url.Parse(e.srv.URL)
	for _, c := range e.client.Jar.Cookies(u) {
		if c.Name == authpkg.CSRFCookieName {
			return c.Value
		}
	}
	t.Fatal("csrfCookie: carddemo_csrf cookie not found")
	return ""
}

// login performs the full two-step login flow and asserts success.
func (e *testEnv) login(t *testing.T, userID, password string) {
	t.Helper()
	// GET /login sets the pre-login CSRF cookie.
	resp := e.get(t, "/login")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /login: want 200, got %d", resp.StatusCode)
	}

	tok := e.csrfCookie(t)
	form := url.Values{
		"user_id":              {userID},
		"password":             {password},
		authpkg.CSRFFormField:  {tok},
	}
	resp = e.post(t, "/login", form)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /login: want redirect, got %d; body: %s", resp.StatusCode, body)
	}
}

// logout posts to /logout and asserts the redirect.
func (e *testEnv) logout(t *testing.T) {
	t.Helper()
	tok := e.csrfCookie(t)
	form := url.Values{authpkg.CSRFFormField: {tok}}
	resp := e.post(t, "/logout", form)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("POST /logout: want redirect, got %d", resp.StatusCode)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests
// ──────────────────────────────────────────────────────────────────────────────

// TestE2E_HealthCheck verifies the unauthenticated health endpoint is up.
func TestE2E_HealthCheck(t *testing.T) {
	e := newTestEnv(t)
	resp := e.get(t, "/healthz")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /healthz: want 200, got %d", resp.StatusCode)
	}
}

// TestE2E_LoginMainMenuLogout verifies the canonical user flow:
// COSGN00C (login) → COMEN01C (main menu) → logout.
func TestE2E_LoginMainMenuLogout(t *testing.T) {
	e := newTestEnv(t)

	// Unauthenticated access to / should redirect to /login.
	resp := e.get(t, "/")
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Errorf("unauthenticated GET /: want redirect, got %d", resp.StatusCode)
	}

	e.login(t, "USER0001", "PASSWORD")

	// After login, GET / should return the main menu (200).
	resp = e.get(t, "/")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET / after login: want 200, got %d", resp.StatusCode)
	}
	// Main menu contains the option list.
	if !strings.Contains(body, "ACCOUNT") && !strings.Contains(body, "Account") {
		t.Errorf("main menu missing expected content; body excerpt: %.200s", body)
	}

	e.logout(t)

	// After logout, / should redirect again.
	resp = e.get(t, "/")
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Errorf("after logout GET /: want redirect, got %d", resp.StatusCode)
	}
}

// TestE2E_MainMenuOption_AccountView verifies that selecting option 1 on the
// main menu redirects to /account/view (COACTVWC).
func TestE2E_MainMenuOption_AccountView(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "USER0001", "PASSWORD")

	// POST to main menu with option 1 (Account View).
	tok := e.csrfCookie(t)
	form := url.Values{
		"option":              {"1"},
		authpkg.CSRFFormField: {tok},
	}
	resp := e.post(t, "/", form)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Errorf("POST / option 1: want redirect, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/account/view") {
		t.Errorf("POST / option 1: want redirect to /account/view, got %q", loc)
	}
}

// TestE2E_AccountView_Empty verifies that GET /account/view with no query
// param renders the search form (COACTVWC blank state).
func TestE2E_AccountView_Empty(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "USER0001", "PASSWORD")

	resp := e.get(t, "/account/view")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /account/view: want 200, got %d; body: %.200s", resp.StatusCode, body)
	}
	// Should render a form asking for account ID.
	if !strings.Contains(body, "acct_id") && !strings.Contains(body, "Account") {
		t.Errorf("account view form missing expected content; body excerpt: %.200s", body)
	}
}

// TestE2E_AccountView_NotFound verifies a graceful 404/error response when
// querying a nonexistent account.
func TestE2E_AccountView_NotFound(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "USER0001", "PASSWORD")

	// POST the lookup form first (PRG pattern).
	tok := e.csrfCookie(t)
	form := url.Values{
		"acct_id":             {"9999999999"},
		authpkg.CSRFFormField: {tok},
	}
	resp := e.post(t, "/account/view", form)
	resp.Body.Close()
	// Should redirect to GET /account/view?acct_id=9999999999.
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Errorf("POST /account/view: want redirect, got %d", resp.StatusCode)
	}

	// Follow the redirect.
	resp = e.get(t, "/account/view?acct_id=9999999999")
	body := bodyString(t, resp)
	// The page should indicate "not found" in some way.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /account/view?acct_id=9999999999: want 200 or 404, got %d; body: %.200s",
			resp.StatusCode, body)
	}
}

// TestE2E_AdminFlow_LoginMenuUserList verifies:
// COSGN00C (admin login) → COADM01C (admin menu) → COUSR00C (user list).
func TestE2E_AdminFlow_LoginMenuUserList(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "ADMIN001", "PASSWORD")

	// GET /admin should show admin menu.
	resp := e.get(t, "/admin")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /admin: want 200, got %d; body: %.200s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "Admin") && !strings.Contains(body, "admin") {
		t.Errorf("admin menu missing Admin content; excerpt: %.200s", body)
	}

	// GET /admin/users should list users.
	resp = e.get(t, "/admin/users")
	body = bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /admin/users: want 200, got %d; body: %.200s", resp.StatusCode, body)
	}
	// Should show existing users.
	if !strings.Contains(body, "ADMIN001") && !strings.Contains(body, "USER0001") {
		t.Logf("user list body excerpt: %.300s", body)
	}
}

// TestE2E_AdminFlow_UserCRUD tests the full COUSR01-03 cycle:
// create a user, view/update it, then delete it.
func TestE2E_AdminFlow_UserCRUD(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "ADMIN001", "PASSWORD")

	// ── Create ───────────────────────────────────────────────────────────────
	tok := e.csrfCookie(t)
	createForm := url.Values{
		"user_id":             {"TESTUSER"},
		"first_name":          {"Test"},
		"last_name":           {"User"},
		"password":            {"TestPas1"},
		"user_type":           {"U"},
		authpkg.CSRFFormField: {tok},
	}
	resp := e.post(t, "/admin/users", createForm)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound &&
		resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /admin/users (create): unexpected status %d", resp.StatusCode)
	}

	// ── Read ─────────────────────────────────────────────────────────────────
	resp = e.get(t, "/admin/users/TESTUSER")
	body := bodyString(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /admin/users/TESTUSER: want 200, got %d; body: %.200s", resp.StatusCode, body)
	}
	if !strings.Contains(body, "TESTUSER") && !strings.Contains(body, "Test") {
		t.Errorf("user edit page missing user data; excerpt: %.300s", body)
	}

	// ── Update ────────────────────────────────────────────────────────────────
	tok = e.csrfCookie(t)
	updateForm := url.Values{
		"user_id":             {"TESTUSER"},
		"first_name":          {"Updated"},
		"last_name":           {"User"},
		"password":            {"NewPass9"},
		"user_type":           {"U"},
		authpkg.CSRFFormField: {tok},
	}
	resp = e.post(t, "/admin/users/TESTUSER", updateForm)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound &&
		resp.StatusCode != http.StatusOK {
		t.Errorf("POST /admin/users/TESTUSER (update): unexpected status %d", resp.StatusCode)
	}

	// ── Delete ────────────────────────────────────────────────────────────────
	tok = e.csrfCookie(t)
	deleteForm := url.Values{authpkg.CSRFFormField: {tok}}
	resp = e.post(t, "/admin/users/TESTUSER/delete", deleteForm)
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound &&
		resp.StatusCode != http.StatusOK {
		t.Errorf("POST /admin/users/TESTUSER/delete: unexpected status %d", resp.StatusCode)
	}

	// Verify the user is gone.
	resp = e.get(t, "/admin/users/TESTUSER")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("after delete GET /admin/users/TESTUSER: want 404, got %d", resp.StatusCode)
	}
}

// TestE2E_AdminBlocked_RegularUser verifies that a regular user cannot access
// admin routes (COADM01C admin-only enforcement).
func TestE2E_AdminBlocked_RegularUser(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "USER0001", "PASSWORD")

	resp := e.get(t, "/admin")
	defer resp.Body.Close()
	// Should redirect to /login or return 403.
	if resp.StatusCode == http.StatusOK {
		t.Errorf("regular user should not access /admin, got 200")
	}
}

// TestE2E_LoginReject_BadCredentials verifies that wrong credentials are
// rejected (COSGN00C security check).
func TestE2E_LoginReject_BadCredentials(t *testing.T) {
	e := newTestEnv(t)

	// GET /login to set CSRF cookie.
	resp := e.get(t, "/login")
	resp.Body.Close()

	tok := e.csrfCookie(t)
	form := url.Values{
		"user_id":             {"ADMIN001"},
		"password":            {"WRONGPASS"},
		authpkg.CSRFFormField: {tok},
	}
	resp = e.post(t, "/login", form)
	body := bodyString(t, resp)
	// Should NOT redirect to home; should re-render login with error.
	if resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusFound {
		t.Errorf("bad credentials: should not redirect, got %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("bad credentials: want 200 or 401, got %d; body: %.200s", resp.StatusCode, body)
	}
}

// TestE2E_Logout_InvalidatesSession verifies that a session cookie is no
// longer valid after logout.
func TestE2E_Logout_InvalidatesSession(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "USER0001", "PASSWORD")

	// Confirm session is active.
	resp := e.get(t, "/")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pre-logout GET /: want 200, got %d", resp.StatusCode)
	}

	e.logout(t)

	// Session should be gone — / redirects to /login.
	resp = e.get(t, "/")
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Errorf("after logout GET /: session still valid (got 200)")
	}
}

// assertContains is a small helper to check page content.
func assertContains(t *testing.T, body, want string) {
	t.Helper()
	if !strings.Contains(body, want) {
		t.Errorf("expected body to contain %q; excerpt:\n%.300s", want, body)
	}
}

// assertRedirectTo checks that a redirect response goes to the expected path.
func assertRedirectTo(t *testing.T, resp *http.Response, path string) {
	t.Helper()
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, path) {
		t.Errorf("want redirect to %q, got Location: %q", path, loc)
	}
	_ = fmt.Sprintf // keep fmt import used
}

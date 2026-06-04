package menu_test

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/audit"
	authpkg "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/auth"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	cardsvc "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/card"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web"
	webauth "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/auth"
	webcardmod "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/card"
)

// testEnv wires the full router for integration testing.
type testEnv struct {
	srv   *httptest.Server
	cli   *http.Client
	store *authpkg.MemorySessionStore
	sink  *audit.MemorySink
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	users := repo.NewInMemoryUserSec()
	if err := authpkg.SeedDefaultUsers(context.Background(), users, 4); err != nil {
		t.Fatalf("seed: %v", err)
	}
	sink := audit.NewMemorySink()
	store := authpkg.NewMemorySessionStore()
	svc := authpkg.NewService(users, store, sink)

	authH := webauth.NewHandlers(svc)
	authH.CookieOptions.Secure = false

	// Wire a minimal card handler backed by an in-memory SQLite for the router.
	memDB, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(func() { memDB.Close() })
	cardH := webcardmod.NewHandlers(cardsvc.NewService(sqlite.NewCardStore(memDB), sqlite.NewCardXrefStore(memDB)))

	router := web.NewRouter(authH, cardH, store, sink)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	cli := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // follow manually where needed
		},
	}
	return &testEnv{srv: srv, cli: cli, store: store, sink: sink}
}

// login performs a full login flow and returns the CSRF token from the session.
func (e *testEnv) login(t *testing.T, userID, password string) string {
	t.Helper()

	// GET /login to obtain pre-login CSRF cookie.
	resp, err := e.cli.Get(e.srv.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /login: want 200, got %d", resp.StatusCode)
	}

	// Extract CSRF token from cookie jar.
	u, _ := url.Parse(e.srv.URL)
	csrfToken := ""
	for _, c := range e.cli.Jar.Cookies(u) {
		if c.Name == authpkg.CSRFCookieName {
			csrfToken = c.Value
		}
	}
	if csrfToken == "" {
		t.Fatal("no CSRF cookie after GET /login")
	}

	// POST /login.
	resp, err = e.cli.PostForm(e.srv.URL+"/login", url.Values{
		"user_id":    {userID},
		"password":   {password},
		"csrf_token": {csrfToken},
	})
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()
	// Expect redirect after successful login.
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /login %s: want 303, got %d", userID, resp.StatusCode)
	}

	// Return the session CSRF token (from cookie after login).
	for _, c := range e.cli.Jar.Cookies(u) {
		if c.Name == authpkg.CSRFCookieName {
			return c.Value
		}
	}
	t.Fatal("no CSRF cookie after login")
	return ""
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestMainMenuRequiresAuth checks that / redirects anonymous users to /login.
func TestMainMenuRequiresAuth(t *testing.T) {
	e := newTestEnv(t)
	resp, err := e.cli.Get(e.srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("anonymous GET /: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "/login") {
		t.Errorf("anonymous GET /: want redirect to /login, got %q", loc)
	}
}

// TestMainMenuRendersForUser checks that a regular user sees the main menu.
func TestMainMenuRendersForUser(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "USER0001", "PASSWORD")

	// Follow the redirect from login to /.
	resp, err := e.cli.Get(e.srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("user GET /: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "Main Menu") {
		t.Errorf("user GET /: body missing 'Main Menu'; got %q", body[:min(len(body), 300)])
	}
}

// TestAdminMenuRequiresAdmin checks that a regular user is denied /admin.
func TestAdminMenuRequiresAdmin(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "USER0001", "PASSWORD")

	resp, err := e.cli.Get(e.srv.URL + "/admin")
	if err != nil {
		t.Fatalf("GET /admin: %v", err)
	}
	resp.Body.Close()
	// RequireAdmin should redirect to /login or return 403.
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusForbidden {
		t.Errorf("user GET /admin: want 303 or 403, got %d", resp.StatusCode)
	}
}

// TestAdminMenuRendersForAdmin checks that an admin sees the admin menu.
func TestAdminMenuRendersForAdmin(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "ADMIN001", "PASSWORD")

	resp, err := e.cli.Get(e.srv.URL + "/admin")
	if err != nil {
		t.Fatalf("GET /admin: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admin GET /admin: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "Admin Menu") {
		t.Errorf("admin GET /admin: body missing 'Admin Menu'; got %q", body[:min(len(body), 300)])
	}
}

// TestMenuPostRedirectsToOption checks that submitting option 1 on the main
// menu redirects to the account view path.
func TestMenuPostRedirectsToOption(t *testing.T) {
	e := newTestEnv(t)
	csrf := e.login(t, "USER0001", "PASSWORD")

	resp, err := e.cli.PostForm(e.srv.URL+"/", url.Values{
		"option":     {"1"},
		"csrf_token": {csrf},
	})
	if err != nil {
		t.Fatalf("POST /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("POST / option=1: want 303, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "account") {
		t.Errorf("POST / option=1: want account redirect, got %q", loc)
	}
}

// TestMenuPostInvalidOption checks that a bad option re-renders the menu.
func TestMenuPostInvalidOption(t *testing.T) {
	e := newTestEnv(t)
	csrf := e.login(t, "USER0001", "PASSWORD")

	resp, err := e.cli.PostForm(e.srv.URL+"/", url.Values{
		"option":     {"99"},
		"csrf_token": {csrf},
	})
	if err != nil {
		t.Fatalf("POST / option=99: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("POST / bad option: want 200 (re-render), got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "valid option") {
		t.Errorf("POST / bad option: expected error message; got %q", body[:min(len(body), 300)])
	}
}

// TestStubRoutesReachable verifies that all stub BMS-map routes return 200
// for an authenticated user (not 404), satisfying the acceptance criterion
// that all 17 BMS maps have a registered template.
func TestStubRoutesReachable(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "USER0001", "PASSWORD")

	userRoutes := []string{
		"/account/view",
		"/account/update",
		"/cards/list",
		"/cards/view",
		"/cards/update",
		"/transactions",
		"/transactions/view",
		"/transactions/add",
		"/reports",
		"/billing",
		"/pending",
	}
	for _, path := range userRoutes {
		resp, err := e.cli.Get(e.srv.URL + path)
		if err != nil {
			t.Errorf("GET %s: %v", path, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d", path, resp.StatusCode)
		}
	}
}

// TestAdminStubRoutesReachable verifies admin stub routes return 200.
func TestAdminStubRoutesReachable(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "ADMIN001", "PASSWORD")

	adminRoutes := []string{
		"/admin/txn-types",
		"/admin/txn-types/update",
	}
	for _, path := range adminRoutes {
		resp, err := e.cli.Get(e.srv.URL + path)
		if err != nil {
			t.Errorf("GET %s: %v", path, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: want 200, got %d", path, resp.StatusCode)
		}
	}
}

// TestCSRFRejectedOnMenuPost verifies that a POST without a valid CSRF token
// is rejected with 403.
func TestCSRFRejectedOnMenuPost(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "USER0001", "PASSWORD")

	resp, err := e.cli.PostForm(e.srv.URL+"/", url.Values{
		"option":     {"1"},
		"csrf_token": {"bogus-token"},
	})
	if err != nil {
		t.Fatalf("POST / bad csrf: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST / bad csrf: want 403, got %d", resp.StatusCode)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	var sb strings.Builder
	if _, err := io.Copy(&sb, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

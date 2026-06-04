package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/audit"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/auth"
	webauth "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/auth"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/layout"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/menu"
	webtxn "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/transaction"
)

// NewRouter builds and returns the chi router for the CardDemo web server.
// Route groups map to BMS screen clusters:
//
//	Public:     GET /login, POST /login              (COSGN00)
//	User:       GET /, POST /                        (COMEN01)
//	            GET /account/*, /cards/*, /txn/*, …  (stub — per-module PRs)
//	Admin:      GET /admin, POST /admin              (COADM01)
//	            /admin/users/*                       (COUSR00-03)
//
// Auth middleware is applied per group so anonymous requests are never
// silently allowed into protected routes.
//
// txnH may be nil, in which case transaction routes serve the "under
// construction" stub.
func NewRouter(
	authH *webauth.Handlers,
	store auth.SessionStore,
	auditSink audit.Sink,
	txnH *webtxn.Handlers,
) http.Handler {
	r := chi.NewRouter()

	// Global: structured recovery + session loader (no-ops for anonymous).
	r.Use(middleware.Recoverer)
	r.Use(auth.LoadSession(store))

	// Static assets — no auth required.
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Health check — unauthenticated, used by load balancers.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// ── Public routes (sign-on / sign-off) ───────────────────────────────────
	// CSRFProtect is applied here; GetLogin generates a pre-login CSRF token.
	r.Group(func(r chi.Router) {
		r.Use(auth.CSRFProtect())
		r.Get("/login", authH.GetLogin)
		r.Post("/login", authH.PostLogin)
		r.Post("/logout", authH.PostLogout)
	})

	// ── Authenticated user routes ─────────────────────────────────────────────
	// RequireUser redirects unauthenticated requests to /login.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireUser("/login"))
		r.Use(auth.CSRFProtect())

		// COMEN01 — main menu (GET shows it, POST dispatches the selected option).
		r.Get("/", menu.GetMainMenu)
		r.Post("/", menu.PostMainMenu)

		// Per-module stubs: RAU-40 (account), RAU-41 (card), RAU-45 (report).
		// Transaction + bill-payment routes are implemented by txnH (RAU-42).
		r.Get("/account/view", stubHandler("Account View"))
		r.Get("/account/update", stubHandler("Account Update"))
		r.Get("/cards/list", stubHandler("Credit Card List"))
		r.Get("/cards/view", stubHandler("Credit Card View"))
		r.Get("/cards/update", stubHandler("Credit Card Update"))
		r.Get("/reports", stubHandler("Transaction Reports"))
		r.Get("/pending", stubHandler("Pending Authorization View"))

		// COTRN00C / COTRN01C / COTRN02C / COBIL00C — transaction + bill pay.
		if txnH != nil {
			r.Get("/transactions", txnH.ListTxns)
			r.Get("/transactions/view", txnH.ViewTxn)
			r.Get("/transactions/add", txnH.GetAddTxn)
			r.Post("/transactions/add", txnH.PostAddTxn)
			r.Get("/billing", txnH.GetBillPay)
			r.Post("/billing", txnH.PostBillPay)
		} else {
			r.Get("/transactions", stubHandler("Transaction List"))
			r.Get("/transactions/view", stubHandler("Transaction View"))
			r.Get("/transactions/add", stubHandler("Transaction Add"))
			r.Get("/billing", stubHandler("Bill Payment"))
		}
	})

	// ── Admin-only routes ─────────────────────────────────────────────────────
	// RequireAdmin rejects non-admin sessions (logged to audit sink).
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAdmin(auditSink, "/login"))
		r.Use(auth.CSRFProtect())

		// COADM01 — admin menu.
		r.Get("/admin", menu.GetAdminMenu)
		r.Post("/admin", menu.PostAdminMenu)

		// COUSR00-03 — user CRUD (RAU-39, done).
		r.Get("/admin/users", authH.ListUsers)
		r.Post("/admin/users", authH.CreateUser)
		r.Get("/admin/users/new", authH.GetUser) // shows blank form
		r.Get("/admin/users/{id}", authH.GetUser)
		r.Post("/admin/users/{id}", authH.UpdateUser)
		r.Post("/admin/users/{id}/delete", authH.DeleteUser)

		// Admin stubs — transaction type screens (Db2 module, future issue).
		r.Get("/admin/txn-types", stubHandler("Transaction Type List"))
		r.Get("/admin/txn-types/update", stubHandler("Transaction Type Maintenance"))
	})

	return r
}

const stubContent = `{{define "content"}}
<section class="stub-screen">
  <p class="stub-notice">This screen is under construction ({{.Title}}).</p>
  <p><a href="javascript:history.back()">&#8592; Back</a></p>
</section>
{{end}}`

// stubHandler returns a handler that renders a "coming soon" placeholder for
// BMS maps whose owning feature issue has not yet landed. All 17 BMS map
// routes are registered in the router; stub handlers satisfy the acceptance
// criterion that every template is reachable.
func stubHandler(title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		layout.Render(w, layout.NewPage(r, title), stubContent)
	}
}

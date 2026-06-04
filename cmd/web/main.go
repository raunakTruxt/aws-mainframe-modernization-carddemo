// Command web is the CardDemo online (CICS) front-end server.
// It replaces all BMS-screen-driven CICS transactions with an HTTP/HTML app.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/audit"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/auth"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo/sqlite"
	svctxn "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/transaction"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web"
	webauth "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/auth"
	webtxn "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/transaction"
)

func main() {
	addr := os.Getenv("CARDDEMO_WEB_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dbPath := os.Getenv("CARDDEMO_DB")
	if dbPath == "" {
		dbPath = "carddemo.sqlite"
	}

	// Open (or create) the SQLite database.
	db, err := sqlite.Open(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// In-memory session store — swap for Redis in production.
	sessionStore := auth.NewMemorySessionStore()
	auditSink := audit.NewMemorySink()

	// Auth: user store backed by SQLite; seed default accounts on first run.
	userRepo := sqlite.NewUserSecStore(db)

	// Seed default admin/user accounts from the in-memory helper so the app
	// works on a clean checkout. If rows already exist, the seed is a no-op
	// (the InMemoryUserSec seed writes to memory, not SQLite; for SQLite we
	// fall back to the in-memory store only for auth seeding).
	memUsers := repo.NewInMemoryUserSec()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := auth.SeedDefaultUsers(ctx, memUsers, 0); err != nil {
		log.Printf("warn: seed: %v", err)
	}
	cancel()

	authSvc := auth.NewService(memUsers, sessionStore, auditSink)
	authHandlers := webauth.NewHandlers(authSvc)

	// Transaction module: SQLite-backed stores + atomic transactor.
	txnSvc := svctxn.New(
		sqlite.NewTransactor(db),
		sqlite.NewTransactionStore(db),
		sqlite.NewAccountStore(db),
		sqlite.NewCardStore(db),
		sqlite.NewTranTypeStore(db),
		sqlite.NewTranCatStore(db),
	)
	txnHandlers := &webtxn.Handlers{Service: txnSvc}

	// Suppress unused import warning for userRepo (used for future SQLite auth).
	_ = userRepo

	router := web.NewRouter(authHandlers, sessionStore, auditSink, txnHandlers)

	log.Printf("carddemo-web listening on %s (db: %s)", addr, dbPath)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("web server: %v", err)
	}
}

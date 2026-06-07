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
	svcaccount "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/account"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web"
	webaccount "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/account"
	webauth "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/auth"
)

func main() {
	addr := os.Getenv("CARDDEMO_WEB_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	dbPath := os.Getenv("CARDDEMO_DB")
	if dbPath == "" {
		dbPath = "carddemo.db"
	}

	// Open SQLite database (creates + migrates on first run).
	db, err := sqlite.Open(dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	// Repository implementations.
	acctRepo := sqlite.NewAccountStore(db)
	custRepo := sqlite.NewCustomerStore(db)
	xrefRepo := sqlite.NewCardXrefStore(db)

	// In-memory user/session stores — swap for persistent backends in production.
	userRepo := repo.NewInMemoryUserSec()
	sessionStore := auth.NewMemorySessionStore()
	auditSink := audit.NewMemorySink()

	authSvc := auth.NewService(userRepo, sessionStore, auditSink)

	// Seed default admin/user accounts.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := auth.SeedDefaultUsers(ctx, userRepo, 0); err != nil {
		log.Printf("warn: seed: %v", err)
	}
	cancel()

	// Service layer.
	accountSvc := svcaccount.New(acctRepo, custRepo, xrefRepo, svcaccount.Transactor(func(ctx context.Context, fn func(repo.AccountRepository, repo.CustomerRepository) error) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := fn(sqlite.NewAccountStore(tx), sqlite.NewCustomerStore(tx)); err != nil {
			tx.Rollback()
			return err
		}
		return tx.Commit()
	}))

	// HTTP handlers.
	authHandlers := webauth.NewHandlers(authSvc)
	accountHandlers := webaccount.NewHandlers(accountSvc)

	router := web.NewRouter(authHandlers, accountHandlers, nil, sessionStore, auditSink)

	log.Printf("carddemo-web listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("web server: %v", err)
	}
}

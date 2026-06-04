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
	cardsvc "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/service/card"
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web"
	webauth "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/auth"
	webcardmod "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/card"
)

func main() {
	addr := os.Getenv("CARDDEMO_WEB_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dbPath := os.Getenv("CARDDEMO_DB_PATH")
	if dbPath == "" {
		dbPath = "carddemo.sqlite"
	}

	// Open (or create) the SQLite database.
	db, err := sqlite.Open(dbPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	// In-memory stores — swap for Redis + Postgres in production.
	userRepo := repo.NewInMemoryUserSec()
	sessionStore := auth.NewMemorySessionStore()
	auditSink := audit.NewMemorySink()

	authSvc := auth.NewService(userRepo, sessionStore, auditSink)

	// Seed default admin/user accounts so the app works immediately on a clean
	// checkout (development convenience; replaces cleartext COBOL fixtures with
	// bcrypt hashes).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := auth.SeedDefaultUsers(ctx, userRepo, 0); err != nil {
		log.Printf("warn: seed: %v", err)
	}
	cancel()

	// Card service wired to SQLite-backed repositories.
	cardStore := sqlite.NewCardStore(db)
	xrefStore := sqlite.NewCardXrefStore(db)
	cardService := cardsvc.NewService(cardStore, xrefStore)

	authHandlers := webauth.NewHandlers(authSvc)
	cardHandlers := webcardmod.NewHandlers(cardService)
	router := web.NewRouter(authHandlers, cardHandlers, sessionStore, auditSink)

	log.Printf("carddemo-web listening on %s (db: %s)", addr, dbPath)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("web server: %v", err)
	}
}

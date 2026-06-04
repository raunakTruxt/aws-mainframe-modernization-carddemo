// Command web is the CardDemo online (CICS) front-end server.
// It replaces all BMS-screen-driven CICS transactions with an HTTP/HTML app.
// RAU-43 wires the chi router, shared layout, and all 17 BMS-map routes.
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
	"github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web"
	webauth "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/web/auth"
)

func main() {
	addr := os.Getenv("CARDDEMO_WEB_ADDR")
	if addr == "" {
		addr = ":8080"
	}

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

	authHandlers := webauth.NewHandlers(authSvc)
	router := web.NewRouter(authHandlers, sessionStore, auditSink)

	log.Printf("carddemo-web listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("web server: %v", err)
	}
}

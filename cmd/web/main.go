// Command web is the CardDemo online (CICS) front-end server.
// It replaces all BMS-screen-driven CICS transactions with an HTTP/HTML app.
// RAU-43 (web layer) wires in the chi router and per-screen handlers;
// this stub boots a minimal health-check server so the binary compiles now.
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	addr := os.Getenv("CARDDEMO_WEB_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("carddemo-web listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("web server: %v", err)
	}
}

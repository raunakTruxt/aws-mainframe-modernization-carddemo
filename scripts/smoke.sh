#!/usr/bin/env bash
# smoke.sh — end-to-end smoke test.
# Builds binaries, seeds a temp SQLite DB from EBCDIC fixtures, runs the
# batch pipeline, spins up the web server, health-checks it, then cleans up.
# Exits 0 on success, non-zero on any failure.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

SMOKE_DB="$(mktemp /tmp/carddemo-smoke-XXXXXX.sqlite)"
WEB_PID=""

cleanup() {
  if [ -n "$WEB_PID" ] && kill -0 "$WEB_PID" 2>/dev/null; then
    kill "$WEB_PID" 2>/dev/null || true
  fi
  rm -f "$SMOKE_DB"
}
trap cleanup EXIT

echo "==> smoke: building binaries…"
go build -o /tmp/carddemo-seed  ./cmd/seed
go build -o /tmp/carddemo-batch ./cmd/batch
go build -o /tmp/carddemo-web   ./cmd/web

echo "==> smoke: seeding DB from EBCDIC fixtures…"
/tmp/carddemo-seed -db "$SMOKE_DB" -data app/data/EBCDIC

echo "==> smoke: tran-post (CBTRN02C)…"
# Exit code 4 is expected when there are reject records (mirrors COBOL RETURN-CODE=4).
/tmp/carddemo-batch tran-post \
  -db "$SMOKE_DB" \
  -input app/data/EBCDIC/AWS.M2.CARDDEMO.DALYTRAN.PS || {
  RC=$?
  if [ "$RC" -ne 4 ]; then
    echo "smoke: FAIL — tran-post exited with unexpected code $RC" >&2
    exit "$RC"
  fi
}

echo "==> smoke: intcalc (CBACT04C)…"
/tmp/carddemo-batch intcalc -db "$SMOKE_DB"

echo "==> smoke: tran-report (CBTRN03C)…"
/tmp/carddemo-batch tran-report -db "$SMOKE_DB" -output /dev/null

echo "==> smoke: starting web server…"
CARDDEMO_DB="$SMOKE_DB" /tmp/carddemo-web &
WEB_PID=$!

# Wait for the server to be ready (up to 5 seconds).
READY=0
for i in $(seq 1 10); do
  if curl -sf http://localhost:8080/healthz >/dev/null 2>&1; then
    READY=1
    break
  fi
  sleep 0.5
done

if [ "$READY" -ne 1 ]; then
  echo "smoke: FAIL — web server did not become ready within 5s" >&2
  exit 1
fi

HTTP_CODE="$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/healthz)"
if [ "$HTTP_CODE" != "200" ]; then
  echo "smoke: FAIL — GET /healthz returned $HTTP_CODE, want 200" >&2
  exit 1
fi

echo "==> smoke: all checks passed"

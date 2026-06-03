.PHONY: build test lint vet tidy run-web run-batch seed-sqlite clean help

# Set RACE=1 to enable the race detector (requires CGO; skip for sqlite-backed tests).
RACE   ?=
GOTEST := go test $(if $(RACE),-race,) ./...

## build: compile all binaries
build:
	go build ./...

## test: run the full test suite
test:
	$(GOTEST)

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## vet: run go vet
vet:
	go vet ./...

## tidy: update go.sum and remove unused dependencies
tidy:
	go mod tidy

## run-web: start the CardDemo online (CICS) web server
run-web:
	go run ./cmd/web

## run-batch: print batch subcommand usage
run-batch:
	go run ./cmd/batch

## seed-sqlite: load EBCDIC fixtures into carddemo.sqlite (pass -wipe=false to preserve existing data)
seed-sqlite:
	go run ./cmd/seed

## clean: remove build artifacts
clean:
	go clean ./...

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## //'

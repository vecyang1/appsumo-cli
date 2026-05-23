# AppSumo CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a read-only AppSumo buyer-account CLI with strict redaction, product sync, local search, SQL, and live smoke verification.

**Architecture:** Use CLI Printing Press to generate a baseline from the browser-sniffed OpenAPI contract. Keep a hand-authored safety wrapper CLI in Go for AppSumo-specific redaction, pagination, cookie handling, and SQLite commands.

**Tech Stack:** Go 1.26, Cobra, `database/sql`, SQLite driver, CLI Printing Press 4.12.0.

### Task 1: Safety Contract And Redaction

**Files:**
- Create: `internal/redact/redact_test.go`
- Create: `internal/redact/redact.go`

- [x] Write failing tests for CSV and JSON redaction.
- [x] Run `go test ./internal/redact` and verify failure.
- [x] Implement redaction.
- [x] Run `go test ./internal/redact` and verify pass.

### Task 2: AppSumo Client

**Files:**
- Create: `internal/appsumo/client_test.go`
- Create: `internal/appsumo/client.go`
- Create: `internal/appsumo/types.go`

- [x] Write failing tests for cookie auth, products pagination, and CSV fetch.
- [x] Run `go test ./internal/appsumo` and verify failure.
- [x] Implement client and types.
- [x] Run `go test ./internal/appsumo` and verify pass.

### Task 3: SQLite Store

**Files:**
- Create: `internal/store/store_test.go`
- Create: `internal/store/store.go`

- [x] Write failing tests for sync upsert, search, read-only SQL, and SQL write rejection.
- [x] Run `go test ./internal/store` and verify failure.
- [x] Implement store.
- [x] Run `go test ./internal/store` and verify pass.

### Task 4: CLI Commands

**Files:**
- Create: `internal/cli/root_test.go`
- Create: `internal/cli/root.go`
- Create: `cmd/appsumo/main.go`

- [x] Write failing CLI tests for `auth status`, `products list`, `products export`, `sync`, `search`, and `sql`.
- [x] Run `go test ./internal/cli` and verify failure.
- [x] Implement CLI.
- [x] Run `go test ./internal/cli` and verify pass.

### Task 5: Printing Press And Verification

**Files:**
- Generated: `generated/appsumo-account-pp-cli/`
- Modify: `docs/01_appsumo_api_discovery.md`
- Modify: `README.md`

- [x] Run CLI Printing Press generation from `docs/openapi/appsumo-account.openapi.yaml`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run live read-only smoke via logged-in browser session without printing cookies.
- [x] Run tracked-file secret scan.

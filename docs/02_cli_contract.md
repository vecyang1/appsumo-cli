# CLI Contract

## Read-Only Boundary

The CLI may read AppSumo buyer-account data and write local cache files. It must not call AppSumo endpoints that mutate account state.

Disallowed v1 actions:

- refund
- transfer
- activation
- checkout/cart mutation
- plan changes
- profile or billing updates

## Authentication

Supported auth sources:

1. `APPSUMO_COOKIE`
2. `--cookie-file <path>` pointing to an ignored local file containing a full Cookie header

The CLI may report whether auth is present, whether AppSumo accepted it, and whether the session appears logged in. It must not print cookie values.

Cookie headers may only be sent to `https://appsumo.com` or loopback test hosts. Base URL overrides must not forward real AppSumo cookies to arbitrary hosts.

## Commands

### `auth status`

Checks whether a cookie source exists and whether `GET /api/sessions/current/` returns an authenticated session.

### `products list`

Fetches live AppSumo products, follows pagination, and prints a table or JSON.

### `products search <query>`

Searches live products by name, slug, plan, status, and support email.

### `products export`

Exports live products as JSON or AppSumo CSV.

Defaults:

- `--format json`

Sensitive keys and CSV columns must be replaced with `[REDACTED]`.

### `sync`

Fetches all products and stores them in a local SQLite database.

### `search <query>`

Searches synced products in SQLite.

### `sql <select-query>`

Runs read-only `SELECT` queries against the local SQLite database. Non-`SELECT` SQL is rejected before execution.

## Environment

- `APPSUMO_BASE_URL`: override base URL for tests and local smoke servers.
- `APPSUMO_COOKIE`: full Cookie header.
- `APPSUMO_DB_PATH`: local SQLite database path.

## Current Verification Snapshot

On 2026-05-23, a live read-only smoke run against the logged-in browser session verified:

- browser products API status: 200
- browser products API count matched CLI products count
- CLI synced products count matched local SQLite `select count(*) as count from products`
- CSV export retained the `License Key / Code` header but replaced values with `[REDACTED]`

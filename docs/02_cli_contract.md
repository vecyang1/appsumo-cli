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

### macOS Chrome Cookie Decryption (Helper Reference)

When running on macOS, AppSumo cookies (like `sid` and `csrftoken`) can be extracted from the user's Google Chrome profile using the following decryption rules:
1. **Retrieve the Master Key**: Run `security find-generic-password -w -s "Chrome Safe Storage"` to get the Keychain password (a short base64 string; never paste it anywhere).
2. **Derive the AES Key**: Use PBKDF2 with HMAC-SHA1:
   * **Salt**: `b'saltysalt'`
   * **Iterations**: `1003`
   * **Key Length**: `16` bytes (AES-128)
3. **Decrypt Ciphertext**:
   * Chrome cookie database stores encrypted values with a `v10` prefix (`b'v10'`).
   * The 16 bytes following the prefix are the true IV (`encrypted_value[3:19]`).
   * The ciphertext starts at offset 19 (`encrypted_value[19:]`).
   * Decrypt the ciphertext with the derived key and extracted IV using AES-CBC-128. After stripping PKCS7 padding, the decrypted value starts with a 16-byte garbage block (the decrypted IV block). Slice `[16:]` to get the clean UTF-8 string value.

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

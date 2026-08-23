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

### `reviews <product-slug>`

Collects every public review for a product from `GET /api/v2/deals/{deal_id}/reviews/`,
walking `from` until the API returns an empty window.

Flags: `--limit` (0 fetches all), `--page-size`, `--sort`, `--order`.

Contract:

- This is a public surface. The command must not read `APPSUMO_COOKIE` or `--cookie-file`,
  and must not send a cookie to the product page or the reviews API.
- Pagination must use `from`. The `page` parameter is accepted and ignored by the
  server; walking it returns page one repeatedly.
- A window that yields no new review ids must stop the crawl and warn, not retry.
- Completeness must be reconciled against `meta.total`. When that field is absent,
  `fetch.complete` is `null` (unknown), never `false`.
- Warnings go to stderr so `--json` remains a clean pipe.

See `docs/03_reviews_api_discovery.md` for the endpoint contract and its decoys.

Flag: `--save` also writes the flattened threads to the local database.

### `questions <product-slug>`

Collects every public question thread from `GET /api/v2/deals/{deal_id}/questions/`,
walking `from` until the API returns an empty window. Answers arrive nested under their
question and are not counted against `--limit`.

Flags: `--limit`, `--page-size`, `--sort`, `--order`, `--save`.

Contract: identical to `reviews`, and enforced by the same code — the crawl lives in
`internal/appsumo/threads.go`. Two additional rules specific to this surface:

- A question row's `deal_id` is the deal it was asked on, which for a relaunched product
  is not the deal that was requested. Do not assert equality.
- `resolved` is set on the answering reply, not on the question, and is `null` on many
  answered threads. Answeredness is a property of the thread — whether it has replies.

### `deals list|sync|diff`

Reads the public catalog from `GET /api/v2/deals/esbrowse/`.

Flags: `list` takes `--limit`, `--page-size`, `--sort`; `sync` takes `--page-size`,
`--sort`, `--keep`.

Contract:

- Public surface: no cookie, from any of its sources.
- Every request must carry a `sort`. An empty `--sort` is substituted, not forwarded:
  without one the walk silently returns a subset (305 of 363 measured).
- Deduplicate by slug and reconcile against `meta.total_results`. A page yielding no new
  slugs stops the walk and warns.
- `sync` must refuse to record a walk whose `complete` is `false`, because a short
  snapshot makes the next diff report healthy deals as gone.
- `diff` reports a departed slug as `gone`, never `ended`: the endpoint marks every row it
  serves as current, so it cannot distinguish sold out from expired from delisted.
- `codes_remaining` is reported only when `uses_codes` is true; `percent_claimed` only when
  it is not the `-1` sentinel. Neither may be rendered as `0` when unknown.

### `portfolio`

Rolls up the account from the local database, or from the API with `--live`.

Contract:

- Redemption is derived from `redeem_date`, never from `is_redeemed`. That flag reads
  `false` on every product of a live account, including redeemed ones.
- Products that cannot be redeemed — expired, refunded, or an AppSumo membership
  subscription — must not appear on the awaiting-redemption list.
- A disagreement between `is_redeemed` and `redeem_date`, in either direction, must be
  reported on stderr and in the JSON `warnings` array. Healthy input emits nothing.

See `docs/04_catalog_and_questions_discovery.md` for the catalog and Q&A endpoint contracts.

### Scope note

`reviews`, `questions`, and `deals` are deliberately absent from
`docs/openapi/appsumo-account.openapi.yaml`. That spec is the *account* contract and its
generated baseline assumes cookie auth; adding public endpoints to it would produce a
generated client that attaches credentials to unauthenticated surfaces.

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

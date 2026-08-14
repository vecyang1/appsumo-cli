# AppSumo CLI

Read-only AppSumo CLI. Audit the products you own, roll up redemption and licensing status, track the public deal catalog for price and stock moves, and collect every public review and question for a product — without exposing session cookies or license codes.

This is an unofficial community tool. It is not affiliated with, endorsed by, or supported by AppSumo.

## Status

This repository is intentionally read-only. The CLI does not refund, transfer, activate, change plans, check out, or mutate AppSumo account data.

## Authentication

The buyer-account endpoints AppSumo uses are session-cookie gated. The CLI accepts an AppSumo cookie header through `APPSUMO_COOKIE` or a local ignored cookie file. It never prints the cookie value, and it only sends cookies to `https://appsumo.com` or loopback test hosts.

Public surfaces are exempt in the other direction. `appsumo deals`, `appsumo reviews`, and
`appsumo questions` read public endpoints with the cookie stripped structurally, so they work
with no credentials configured and buyer credentials never reach a crawl.

## Commands

**Your account** (needs a session cookie)

- `appsumo auth status`
- `appsumo portfolio`
- `appsumo products list`
- `appsumo products search <query>`
- `appsumo products export --format csv|json`
- `appsumo sync`

**Public data** (no credentials)

- `appsumo deals list`
- `appsumo deals sync`
- `appsumo deals diff`
- `appsumo reviews <product-slug>`
- `appsumo questions <product-slug>`

**Local database**

- `appsumo search <query>`
- `appsumo sql <select-query>`

CSV and JSON exports always redact license/code fields.

## Account Portfolio

```bash
appsumo portfolio
```

```text
70 products (local)

status         activated 36  expired 33  inactive 1
redemption     not_redeemable 2  redeemed 68

licensing      22 active, 23 transferable
refund window  0 of 70 still refundable
purchases      2023-05-05 to 2025-11-24

nothing awaiting redemption
```

Redemption is derived from `redeem_date`, **not** from the API's `is_redeemed`
flag. That flag reads `false` on all 70 products of a live account, including all
36 the buyer has activated, so a rollup keyed on it reports an action list of 70
items that do not exist. When the flag and the date disagree the command says so
on stderr rather than quietly picking one.

## Public Deal Catalog

```bash
appsumo deals list --json > catalog.json
appsumo deals sync      # record a timestamped snapshot
appsumo deals diff      # what moved since the previous snapshot
```

`deals diff` reports arrivals, departures, price moves, stock moves, and countdown
timers appearing:

```text
changed	alumni-bundle	codes_remaining 1126 -> 1125
new    	some-new-tool 	Some New Tool
gone   	ended-deal    	Ended Deal
```

Two behaviours worth knowing before you build on this:

- **A walk without `sort` silently loses rows.** Measured 2026-08-14: `page`
  alone returned 305 of 363 declared deals with 58 duplicates and no error. The
  sort *value* is ignored — `newest` and `price` give identical results — but its
  presence is what makes the walk complete. The CLI always sends one.
- **`codes_remaining: 0` does not mean sold out.** 152 live deals report 0 while
  having `uses_codes: false`, and no deal that sells codes reports 0. The CLI
  prints nothing rather than inventing "0 codes left". `percent_claimed` is `-1`
  on every deal and is reported as unknown.

See [docs/04_catalog_and_questions_discovery.md](docs/04_catalog_and_questions_discovery.md).

## Public Product Questions

```bash
appsumo questions growify --json > growify-questions.json
appsumo questions growify --limit 20
appsumo questions growify --save     # also store in local SQLite
```

Q&A shares the reviews pagination contract, so the same guards apply. Answers
arrive nested under their question and are not counted against `--limit`.

With `--save`, reviews and questions land in a `product_comments` table with reply
threads flattened, so `appsumo sql` can join them against the products you own:

```bash
appsumo sql "select p.name, count(*) as reviews from products p join product_comments c on c.product_slug = p.slug where c.kind = 'review' group by p.name"
```

## Public Product Reviews

`appsumo reviews <product-slug>` collects every public review for a product. It is
a separate, unauthenticated surface: the command never reads or sends the session
cookie, so it works with no credentials configured.

```bash
./appsumo reviews growify --json > growify-reviews.json
./appsumo reviews growify --limit 20
```

The JSON payload is `{product, fetch, warnings, reviews}` with replies nested
under each review. Check `fetch.complete` before analyzing: `true` means the crawl
reconciled against the review total, `false` means it was short or capped, and
`null` means the API reported no total so completeness is unknown. Warnings go to
stderr so `--json` stays a clean pipe.

Do not paginate the review pages by URL. `appsumo.com/products/<slug>/reviews/?page=N`
serves the same five reviews at every page number, and the API accepts `page` and
ignores it — the real offset is `from`. See
[docs/03_reviews_api_discovery.md](docs/03_reviews_api_discovery.md).

## Build

Requires Go 1.26.3 or newer.

Install from source:

```bash
go install github.com/vecyang1/appsumo-cli/cmd/appsumo@latest
```

`go install` writes the binary to `$(go env GOPATH)/bin`. If `appsumo` then reports
`command not found`, that directory is not on your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Or build locally and run it from the checkout as `./appsumo`:

```bash
go build -o ./appsumo ./cmd/appsumo
```

## Examples

```bash
APPSUMO_COOKIE="$(cat /path/to/ignored-cookie-file)" ./appsumo auth status --json
APPSUMO_COOKIE="$(cat /path/to/ignored-cookie-file)" ./appsumo products list --json
APPSUMO_COOKIE="$(cat /path/to/ignored-cookie-file)" ./appsumo products export --format csv
APPSUMO_COOKIE="$(cat /path/to/ignored-cookie-file)" ./appsumo sync
./appsumo search letter --json
./appsumo sql "select count(*) as count from products" --json
./appsumo reviews growify --json
./appsumo questions growify --json
./appsumo deals list --json
./appsumo portfolio --json
```

## Printing Press

The read-only AppSumo account contract lives in `docs/openapi/appsumo-account.openapi.yaml`.

The canonical generator command is clean-output-first:

```bash
rm -rf generated/appsumo-account-pp-cli
cli-printing-press generate \
  --spec docs/openapi/appsumo-account.openapi.yaml \
  --name appsumo-account \
  --output generated/appsumo-account-pp-cli \
  --spec-source browser-sniffed \
  --transport browser-http
```

Do not force-add `generated/`. The generated baseline is a local wheel-check artifact and may expose raw private API output if used directly; the hand-authored root CLI is the safe user surface.

The local binary verified during setup:

```text
cli-printing-press 4.12.0
```

The generated baseline passed `cli-printing-press shipcheck --no-live-check` against `docs/openapi/appsumo-account.openapi.yaml`.

## Verification

Local checks:

```bash
go mod verify
go test ./...
go test -race ./...
go vet ./...
govulncheck ./...
go build -o ./appsumo ./cmd/appsumo
./scripts/install_smoke.sh
```

`scripts/install_smoke.sh` installs the CLI into a throwaway directory and runs it
from outside the checkout, so a packaging fault cannot pass as green. It cannot
check your `PATH` — that is a property of the machine, not the artifact.

Live smoke, run from the logged-in Chrome session without printing cookies, verifies:

- browser `/api/v2/account/products/?page=1` returned HTTP 200
- browser API product count matched CLI product count
- CLI synced product count matched SQLite count
- CSV export contained `[REDACTED]`

## Contributing

Keep the public CLI read-only and privacy-preserving. Do not add account-mutating commands, raw license-code output, unredacted exports, HAR files, or real buyer-account metadata to tracked files.

Run these before sending changes:

```bash
go mod verify
go test ./...
go test -race ./...
go vet ./...
govulncheck ./...
```

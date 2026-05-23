# AppSumo CLI

Read-only AppSumo buyer-account CLI for auditing purchased products, redemption status, and local product search without exposing session cookies or license codes.

This is an unofficial community tool. It is not affiliated with, endorsed by, or supported by AppSumo.

## Status

This repository is intentionally read-only. The CLI does not refund, transfer, activate, change plans, check out, or mutate AppSumo account data.

## Authentication

The buyer-account endpoints AppSumo uses are session-cookie gated. The CLI accepts an AppSumo cookie header through `APPSUMO_COOKIE` or a local ignored cookie file. It never prints the cookie value, and it only sends cookies to `https://appsumo.com` or loopback test hosts.

## Commands

- `appsumo auth status`
- `appsumo products list`
- `appsumo products search <query>`
- `appsumo products export --format csv|json`
- `appsumo sync`
- `appsumo search <query>`
- `appsumo sql <select-query>`

CSV and JSON exports always redact license/code fields.

## Build

Requires Go 1.26.3 or newer.

Install from source:

```bash
go install github.com/vecyang1/appsumo-cli/cmd/appsumo@latest
```

Or build locally:

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
go vet ./...
govulncheck ./...
go build -o ./appsumo ./cmd/appsumo
```

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
go vet ./...
govulncheck ./...
```

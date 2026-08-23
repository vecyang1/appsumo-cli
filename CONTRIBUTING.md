# Contributing

Thanks for helping keep this CLI useful and safe.

## Safety Boundary

This project is read-only. Contributions must not add commands or API calls that refund, transfer, activate, change plans, check out, mutate profile/billing state, or otherwise change an AppSumo account.

Do not commit:

- AppSumo cookies or session tokens
- account emails, customer IDs, billing fields, or exact private product counts
- license keys/codes
- raw CSV exports
- HAR files or unredacted browser captures
- generated `generated/` output

## Local Checks

Run:

```bash
go mod verify
go test ./...
go vet ./...
govulncheck ./...
```

For OpenAPI/Printing Press changes, regenerate from a clean output directory and run shipcheck:

```bash
rm -rf generated/appsumo-account-pp-cli
cli-printing-press generate --spec docs/openapi/appsumo-account.openapi.yaml --name appsumo-account --output generated/appsumo-account-pp-cli --spec-source browser-sniffed --transport browser-http --json
cli-printing-press shipcheck --dir generated/appsumo-account-pp-cli --spec docs/openapi/appsumo-account.openapi.yaml --no-live-check --json
```

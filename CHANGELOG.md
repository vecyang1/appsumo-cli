# Changelog

## 0.1.0 - 2026-06-16

- Initialize read-only AppSumo buyer-account CLI project.
- Document AppSumo account endpoint discovery and CLI safety contract.
- Add strict default redaction requirement for license/code export fields.
- Add auth status, products list/search/export, sync, local search, and read-only SQL commands.
- Harden cookie forwarding so base URL overrides cannot receive real AppSumo cookies outside AppSumo or loopback tests.
- Make CSV license/code redaction non-optional and reject the removed `--redact-codes` opt-out.
- Fail explicitly on oversized AppSumo responses instead of silently truncating exports.
- Enforce SQLite `query_only` mode while running user SQL.
- Generate and shipcheck a CLI Printing Press baseline from the read-only OpenAPI contract.
- Verify live AppSumo read-only smoke against the logged-in browser session without printing cookies.
- Prepare the project for community use with public module path, CI, license, and security notes.
- Require Go 1.26.3+ in CI and contributor checks so standard-library vulnerability scans stay clean.
- Document macOS Chrome cookie decryption helper and verify live sync (70 products successfully synced).

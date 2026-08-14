# Changelog

## Unreleased

### Added

- Add `appsumo questions <product-slug>`, which collects every public question thread with answers nested underneath.
- Add `appsumo deals list|sync|diff` over the public catalog, with timestamped snapshots and a diff reporting arrivals, departures, price moves, stock moves, and countdown timers.
- Add `appsumo portfolio`, an account rollup of status, redemption, licensing, and refund window.
- Add `--save` to `reviews` and `questions`, storing flattened comment threads in SQLite so `appsumo sql` can join them against owned products.
- Document the public catalog and Q&A surfaces, their decoy parameters, and their meaningless fields in `docs/04_catalog_and_questions_discovery.md`.
- Add `appsumo reviews <product-slug>`, which collects every public review for a product and nests reply threads.
- Document the public reviews API and its three decoys in `docs/03_reviews_api_discovery.md`: the ignored `page` parameter, the statically generated review page that serves five reviews at every page number, and `/api/v2/deals/?slug=` ignoring the slug.

### Fixed

- Derive redemption from `redeem_date` instead of `is_redeemed`. The API reports `is_redeemed: false` on every product of a live 70-product account, including all 36 the buyer activated, so any rollup keyed on it listed 70 products as awaiting redemption. The disagreement is now reported on stderr rather than silently resolved.
- Always send a `sort` parameter when walking the deal catalog. Without one the backing search returns overlapping pages and drops rows: a full walk collected 305 of 363 declared deals, with 58 duplicates, no error, and a healthy-looking `meta` on every page.
- Report `codes_remaining` and `percent_claimed` as unknown rather than zero. The catalog sends `0` for deals that do not sell codes (152 of 363) and `-1` for every percentage, so a naive read marks available deals sold out.
- Refuse to record an incomplete catalog walk as a snapshot, which would make the next diff report every unserved deal as gone.
- Stop telling public commands their session may be unauthenticated. The message was raised from the shared JSON decoder, so `deals`, `reviews`, and `questions` — none of which ever authenticate — printed an auth diagnosis verbatim, while the account commands that genuinely needed a remedy were given none. The shared code now reports only what came back, and the account commands attach a remedy naming `APPSUMO_COOKIE` and `--cookie-file`, distinguishing "no cookie configured" from "the configured cookie was rejected".
- Mark an anomaly-triggered early stop as truncated, in both the catalog walk and the shared comment crawl. Previously only `--limit` and the request cap set that flag, so a walk that stopped because the source was re-serving windows could still certify itself complete whenever its collected count happened to match the declared total — and `deals sync` reads exactly that verdict to decide whether to persist a snapshot. Reported by a code review; the reconciliation warning is now gated on deliberate caps only, so an anomalous stop still reports its shortfall.
- Distinguish an unreadable licensing flag from a stated false in `appsumo portfolio`. `has_active_license`, `can_transfer_license`, and `is_refundable` are printed as headline counts, and all three decode from an untyped field: a rename would have turned "22 active licenses" into "0" with no error. Unreadable values are now counted and warned about instead of silently counting as false.
- Report the parameters a crawl actually sent rather than the ones requested. `--sort ""` is substituted with the default, so a complete walk was being labelled `sort: ""` — the one setting known to lose 16% of the catalog.
- Serialise an empty `warnings` array as `[]` rather than `null`, so a JSON consumer does not have to special-case healthy output.
- Give catalog snapshots sub-second ids. Two syncs inside the same second shared an id, and the second silently overwrote the first instead of being recorded.
- Strip the session cookie from public product-page and review requests via `Client.public()`.
- Reconcile review crawls against `meta.total` and report unverifiable completeness as unknown rather than false.
- Stop a review crawl that stops yielding new ids instead of looping to the request cap.
- Accept multi-line `SELECT` in `appsumo sql`; the read-only guard tested for the literal `"select "` and rejected any query whose first whitespace was a newline.

### Changed

- Share the offset crawl between reviews and questions (`internal/appsumo/threads.go`), so the duplicate-window guard and the reconciliation against `meta.total` protect both surfaces.

### Security

- Raise the Go floor from 1.26.3 to 1.26.6. go1.26.3 carries seven standard-library vulnerabilities this CLI reaches — `GO-2026-6218` (`net/url`), `GO-2026-6090` and `GO-2026-5856` (`crypto/tls`), `GO-2026-5972` (`encoding/asn1`), `GO-2026-5039` (`net/textproto`), `GO-2026-5037` (`crypto/x509`), and `GO-2026-5026` (`net/http`) — all fixed by 1.26.6. CI installs the exact version in the `go` directive, so the directive is the floor that matters.

### Internal

- Update `modernc.org/sqlite` from v1.50.1 to v1.56.0, with `golang.org/x/sys` and `modernc.org/libc` following. Full suite, install smoke, and a live catalog/review/portfolio run re-verified on the new driver.

- Add a test-binary database sandbox so a CLI test that omits `DBPath` cannot write to the developer's real account database, plus an after-run backstop that fails the suite if it did.
- Add `TestTrackedMarkdownHasNoCredentialShapedLiterals`, which scans every git-known Markdown file for credential-shaped example values. A real Chrome Safe Storage key had reached a release commit as documentation prose, where no cookie or redaction check looks.
- Widen the documented-command gate from the repository root and `docs/` to every git-known Markdown file: 9 files and 22 commands became 17 files and 51 commands.
- Widen `scripts/install_smoke.sh` to grade every top-level command, and point its database at a throwaway path so it cannot write to the developer's real data.
- Add `scripts/install_smoke.sh` to CI, which installs the CLI and runs it from outside the checkout so a packaging fault cannot pass as green.
- Document that `go install` writes to `$(go env GOPATH)/bin`, and give the `command not found` remedy where the install claim is made.
- Add a documentation gate that fails when tracked Markdown documents an `appsumo` command the parser rejects.

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

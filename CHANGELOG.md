# Changelog

## 0.5.5 - 2026-10-08

### Added

- **Fast Local Catalog Query (`deals list --local`)**: Implemented `ListDealsQuery` in `internal/store/deals.go` and added `--local` flag to `appsumo deals list` in `internal/cli/deals.go`, allowing instantaneous (<0.08s), offline-safe full-catalog queries directly from SQLite.
- **Resilient Deal Fetch Pipeline with Auto-Fallback**: Enhanced `appsumo_buy_advisor.py` (`fetch_candidate_deals`) to prefer local SQLite snapshots for catalog audits, with seamless automatic fallback to live remote API when local cache is absent or empty.
- **Unit Test Coverage Expansion**: Added `TestStoreListDealsQuery` in Go (`internal/store/deals_test.go`) and added test cases in Python (`test_fetch_candidate_deals_prefers_local`, `test_fetch_candidate_deals_fallback_to_live_on_empty_local`) bringing unit test count to 33/33 tests passing.

### Fixed

- **Pre-Sync Weekly Skip Gate Optimization**: Optimized `scripts/run_appsumo_buy_advisor.sh` to check the weekly atomic success marker (`YYYY-WW.success`) *before* executing `deals sync`. When skipped, execution completes in 0.11s with 0 network calls (previously 3.4s with 4 redundant HTTP requests).
- **Owner Surface Clarification**: Updated `docs/cadence/CAD-20261007-appsumo-weekly-buy-advisor.md` and 2nd Brain `cadence-commands.md` to cleanly delineate `Output Owner` (Notion Task Page, local SQLite deals snapshot, receipts) from `Source Refs` (`appsumo_purchased_assets.md`), strictly adhering to `scheduled-task-rescheduler` standards.

## 0.5.4 - 2026-10-08

### Changed

- **Cadence Contract & SSOT Multi-Surface Synchronization**: Fully audited and synchronized `CAD-20261007-appsumo-weekly-buy-advisor` across all three authoritative surfaces per `scheduled-task-rescheduler` standards:
  - Project repository card (`docs/cadence/CAD-20261007-appsumo-weekly-buy-advisor.md`): Updated Activation Gate to certify 31/31 Python unit tests and all Go tests passing.
  - 2nd Brain authoritative registry (`cadence-commands.md`): Reconciled `Updated: 2026-10-08`, 66+ owned assets, full live catalog (338+ deals), updated Activation Gate, and synchronized Prompt.
  - Antigravity sidecar projection (`~/.gemini/config/sidecars/appsumo-weekly-buy-advisor/sidecar.json`): Updated prompt in `args[3]` word-for-word with the canonical card, eliminating prompt split-brain drift.
- **Intake & Observability Audit**: Verified catalog sync freshness via `appsumo deals sync`, full pagination verification (338/338 deals across 4 pages), weekly atomic skip marker (`2026-W40.success`), durable state cursor (`state.json`), and live Notion Task Page (`3f2e1b43-2393-815a-97e2-e06c9efcbfdc`) synchronization. Confirmed no extra sidecars needed (skip redundant cadence creation).

## 0.5.3 - 2026-10-08

### Fixed

- **Client-Side Filter Pagination Completeness (`ScannedDeals`)**: Added `ScannedDeals` to Go `DealsResult` and `Complete()` logic, preventing false-positive "⚠️ 分页存在截断" alerts when running filtered scans (e.g. `--scan-mode ideal`) where matching deals < declared total deals.
- **Hardened URL & Query/Fragment Stripping (`CleanProductSlug`)**: Hardened `CleanProductSlug` in both Go and Python to strip fragment identifiers (`#...`) and query strings (`?...`), correctly parsing schemaless URLs (`appsumo.com/products/flipbooklets/?query=test#pricing`) and bare path inputs (`products/flipbooklets?utm_source=email`).
- **Unbounded Store Query Support**: Fixed `IdealDealsQuery` in `internal/store/deals.go` so `limit <= 0` returns all matching deals without being clamped to 20. Added `db.GetDeal(ctx, slug)` for single deal retrieval.
- **Python 3.14 Chunked Encoding Resilience**: Switched `fetch_deal_by_slug` from `urllib.request` to `curl` subprocess, preventing `http.client.IncompleteRead` crashes on AppSumo chunked HTTP responses.
- **Deal Schema Normalization & Title Preservation**: Added `_normalize_scraped_deal` mapping local `data/<slug>/deal.json` and SQLite rows into unified Deal format (preventing `name = None` and `$0.00` price false lead-magnet triggers). Fixed asset title splitting to preserve hyphenated brand names (e.g. `U-xer`), and added short token protection in `is_owned`.
- **Graceful Unresolved Deal Handling**: `audit_deal_steps` handles `is_unresolved` deals by checking existing portfolio ownership first and triggering Rule 6 catalog metadata existence gate if unknown, preventing false Rule 0 trigger.
- **Expanded Test Suite**: Expanded Go and Python test suites to cover schemaless URL parsing, hyphenated name preservation, token collision protection, and unresolved deal audits (31/31 Python tests and all Go tests passing).

## 0.5.2 - 2026-10-08

### Added

- **Full-catalog aggressive crawl (`--limit 0`)**: Eliminated arbitrary 50-deal scan limits. The deal scanning engine and Go CLI now walk across all paginated deal pages (fetching 100 deals/request) to capture the entire active AppSumo catalog (338 deals across 4 pages in ~3s).
- **Pagination completeness reconciliation**: `fetch_candidate_deals` now returns detailed pagination reconciliation metadata (`declared_total`, `unique_deals`, `requests`, `complete`), rigorously asserting that `unique_deals == declared_total` and certifying complete catalog coverage without silent window truncation or cycle loops.
- **Diagnostic-First deal inspector (`explain`)**: Added `appsumo deals explain <product-slug-or-url>` to the Go CLI and `explain` subcommand to `appsumo_buy_advisor.py`, providing operators and agents with an interactive inspection entrypoint. Displays the full deal profile, canonical slug extraction, and a 9-step rule audit chain (Rule 0 through Rule 8) detailing why a deal was accepted or filtered out, which rule was triggered, and what existing asset acts as its replacement.
- **SSOT audit engine (`audit_deal_steps`)**: Consolidated audit logic into `DealAuditor.audit_deal_steps()`, with `audit_deal()` and `explain_deal()` both deriving from the same single source of truth, completely eliminating logic drift or fake facade algorithms.
- **CLI global availability**: Symlinked `~/.local/bin/appsumo-buy-advisor` to `scripts/appsumo_buy_advisor.py` and rebuilt `appsumo` binary in `~/.local/bin/appsumo`, satisfying the Zero "Command Not Found" invariant from any working directory.
- **Register FlipBooklets in 2nd Brain**: Registered `FlipBooklets` in `05 - Memory Center/tools/appsumo_purchased_assets.md` (66 total lifetime assets), mapping `flipbooklets-cli` and Cloudflare edge proxy.

### Fixed

- **Fix unrated deal comparisons (null-handling crash)**: Coerced `average_rating`, `review_count`, and `price` to safe float/int defaults (`float(deal.get("average_rating") or 0.0)`), resolving Python `TypeError` crashes on unrated or newly listed catalog deals (14/338 deals had `None` ratings).
- **Eliminate duplicate wheel leaks beyond top 50 deals**: Saturated all 21 previously leaking deals across the full 338-deal catalog:
  - Mapped `Issuu` -> `FlipBooklets` (intercepting `FlipLink.me`).
  - Mapped `Airtable` / `Monday.com` -> `Boost.space + Logic Sheet + Baserow/Notion` (intercepting `Stackby`).
  - Mapped `UiPath` / `Browse AI` -> `ZeroWork + ego-browser / n8n` (intercepting `RTILA X`).
  - Mapped `Otter.ai` / `Granola` -> `Letterly + Whisper` (intercepting `Prismical`, `Hedy AI`).
  - Filtered non-software/MBA courses via `\bmba\b` token and `No-Code Automation` subcategory (intercepting `No Code MBA`).
  - Added saturated `SEO` subcategory coverage (intercepting `SEO Generator`, `Backlink Monitor`, `WP 301 Redirects`).
- **Zero-duplicate recommendation invariant**: Reconciled and audited all 338/338 deals against 66+ assets with 100% interception rate (0 duplicate recommendations, `no_purchase_needed` verdict).
- **Expand test suite**: Expanded unit test coverage from 19 to 27 tests in `scripts/test_appsumo_buy_advisor.py` covering SSOT step equality, slug extraction, FlipBooklets registration, null resilience, and competitor intercepts.

## 0.5.1 - 2026-10-07

### Fixed

- **Fix duplicate recommendations leak**: Resolved false-positive deal recommendations (ElkQR, Switchy, BeHuman.Online, Measuremate, InstaCharts, TeliportMe, AR/3D viewer) by expanding `COMPETITOR_TO_OWNED_ASSET` with Bitly, Rebrandly, Linktree (mapped to `SleekBio` + `BetterLinks`), Zoom, Hopin, ON24 (mapped to `GoBrunch`), Flourish (mapped to `Logic Sheet` + local viz), and Powtoon (mapped to `Remotion` + `Manim`).
- **Expand saturated subcategories**: Added `Virtual Events`, `Website Analytics`, `Data & Reporting`, `Paid Ads & Retargeting`, `Photo Editing`, and `Design Tools` to `SATURATED_SUBCATEGORIES`, ensuring 50/50 candidate deals accurately pass deduplication and emit a clean 0-recommendation verdict (`no_purchase_needed`).
- **Fix CLI global availability**: Created `~/.local/bin/cadence_ctl` symlink to satisfy the workspace Zero "Command Not Found" invariant, allowing `cadence_ctl validate`, `doctor`, and `test-sidecar` to be executed from any working directory.
- **Fix console output formatting**: Gracefully handle `status == "skipped"` in `main()` without outputting `None 款 | None 款`.
- **Fix Notion block duplication and chunked encoding crash**: Updated `cleanup_previous_notion_runs` to use `curl` instead of `urllib.request`, preventing Python 3.14 `http.client.IncompleteRead` errors and purging 28 outdated test blocks from Notion page `3f2e1b43-2393-815a-97e2-e06c9efcbfdc` to maintain a single authoritative verdict.
- **Expand unit tests**: Added 6 new unit tests (19/19 passing) covering link shorteners, QR tools, virtual events, chart embedders, GA4/GTM wrappers, and out-of-domain checks.

## 0.5.0 - 2026-10-07

### Added

- Add Weekly Buy Advisor engine (`scripts/appsumo_buy_advisor.py`) and runner wrapper (`scripts/run_appsumo_buy_advisor.sh`): automatically audits AppSumo candidate deals against 65+ lifetime purchased assets (`05 - Memory Center/tools/appsumo_purchased_assets.md`).
- Implement zero-recommendation invariant and anti-duplicate guards: features word-boundary token matching, 40+ competitor `alternative_to` mappings (Zapier/Make -> Albato+n8n, Trustpilot -> TrustMate, Buffer -> Zernio, ThriveCart -> Zylvie, OptinMonster -> Taku, etc.), saturated category guards (Reviews, Popups, Scheduling, Forms, Social), Google Sheets AI add-ons, WooCommerce builders, client portals, dev tools, and $0 / non-software item filters. Emits zero recommendations if all deals are duplicate or low leverage.
- Add comprehensive unit test suite `scripts/test_appsumo_buy_advisor.py` (13/13 passing test cases covering exact matches, fuzzy/token normalization, alternative_to aliases, saturated subcategories, free/low-value filters, and zero recommendations).
- Register weekly cadence card `CAD-20261007-appsumo-weekly-buy-advisor` (runs every Monday at 09:00) across 2nd Brain `00 - System/registries/cadence-commands.md`, local documentation `docs/cadence/CAD-20261007-appsumo-weekly-buy-advisor.md`, and launchd sidecar `~/.gemini/config/sidecars/appsumo-weekly-buy-advisor/sidecar.json`.
- Add automated sync of advisory receipts to Notion Goal/Task pages (`3f2e1b43-2393-815a-97e2-e06c9efcbfdc`).

## 0.4.1 - 2026-10-07

### Added

- Add `scripts/select_50_products.py` and `scripts/batch_test_50.py` for automated stratified sampling, stress testing, and diagnostic reconciliation.
- Add `docs/RECONCILIATION_REPORT_50.md` certifying 100% pass rate across 50 diverse products (3,920 reviews, 9,492 questions, 422 FAQs, 171 founder updates, and 92.7 MB structured data).

### Fixed

- Add `FlexBool` and `FlexInt` types in `internal/appsumo/flex.go` to gracefully handle historical AppSumo schema anomalies where boolean fields (`pinned`, `resolved`, `approved`, `edited`, `followup`, `purchased`) contain ISO 8601 timestamp strings (e.g. `"2021-01-26T12:37:42..."` on older products like `fusebase`) or string numbers (`"1"`, `"0"`), preventing deserialization halts.
- Update `internal/store/threads.go` with `flexIntToInt` conversion for safe SQLite storage of upvotes/downvotes.

## 0.4.0 - 2026-10-07

### Added

- Add `appsumo scrape <slug-or-url>` subcommand to automatically aggregate, scrape, save, and cache complete AppSumo product packages (deal specifications, company background, founders, 6 licensing tiers, terms & conditions, FAQs, founder posts, all 161 public reviews, and 184 public questions) into structured JSON files (`deal.json`, `reviews.json`, `questions.json`, `faqs.json`, `founders.json`, `full_archive.json`) and a comprehensive Markdown dossier (`DOSSIER.md` / `SUMMARY.md`).
- Add `CleanProductSlug` to flexibly extract product slugs from arbitrary full URLs (`https://appsumo.com/products/poppy-ai/`), review paths, or plain slug strings.
- Add resilient HTTP client transport with increased TLS handshake timeout (25s) and automatic exponential retry on transient network errors in `getHTML` and `getJSON`.
- Add global PATH exposure via symlink at `~/.local/bin/appsumo`.
- Add standalone Python CLI harness `scripts/appsumo_scraper.py`.

## 0.3.0 - 2026-09-10

### Added

- Add `appsumo deals search <query>` to search live public AppSumo deals or synced local SQLite snapshots (`--local`).
- Add `appsumo deals ideal` to detect and filter ideal deals (rating >= 4.5, review count >= 10, customizable via `--min-rating`, `--min-reviews`, `--query`, `--category`, `--max-price`, and `--local`).
- Add `--chinese` (`-c`) and `--format` (`table|card|markdown`) output modes to `appsumo deals ideal` for rich Chinese product recommendation cards, complete with product links, pricing, discount calculations, value proposition, and competitor comparisons.
- Add `appsumo deals sync-notion` to sync curated ideal deals directly to Notion with customizable thresholds and zero external dependencies.
- Add `--query`, `--min-rating`, `--min-reviews`, `--max-price`, and `--category` flags to `appsumo deals list`.
- Add `--deals` flag to `appsumo search <query>` to query synced catalog deals from local SQLite.
- Capture and persist rich deal metadata: `card_description`, `value_prop`, `best_for`, `alternative_to`, `integrations`, and `subcategory`.
- Add `SearchDeals`, `IdealDeals`, and `IdealDealsQuery` store query APIs with automatic column migrations on SQLite.
- Verified live API contract: AppSumo Elasticsearch browse endpoint `/api/v2/deals/esbrowse/` accepts `query` parameter (and ignores `q`), and supports `sort=rating` for verified highest-rated deals.
- Add `com.vec.appsumo-deal-monitor` LaunchAgent and `scripts/run_appsumo_monitor.sh` for persistent daily catalog tracking and Notion synchronization.

### Fixed

- Fix premature truncation in `FetchAllDealsQuery`: walking candidate deals across pages and deterministically sorting by `(average_rating DESC, review_count DESC, price ASC)` prevents dropping higher-rated products that appear on later pages.
- Decouple pagination cycle detection (`newUnseen == 0`) from client-side filters in `FetchAllDealsQuery`, preventing infinite pagination loops while avoiding false positive early halts.
- Push `--query`, `--category`, and `--max-price` filters down to SQLite in `IdealDealsQuery`, eliminating the bug where in-memory filtering of a pre-limited slice dropped valid matching deals.
- Fix `scripts/sync_notion_recommendations.py` to resolve binary path dynamically, accept CLI arguments (`--page-id`, `--token`, `--min-rating`, `--min-reviews`, `--limit`, `--local`), and execute reliably outside the repository working directory.
- Accurately report local database provenance in CLI output headers instead of misreporting "X of unknown live deals in 0 requests".
- Add `deals search`, `deals ideal`, and `deals sync-notion` to `scripts/install_smoke.sh` gates.

## 0.2.0 - 2026-08-14

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

- Replace the worked example in the macOS cookie-decryption reference with a placeholder. The release commit documented `security find-generic-password -s "Chrome Safe Storage"` with its real output — a key that decrypts every cookie in the machine's Chrome profile, in a repository whose remote is public. It was caught before the commit was ever pushed, and `TestTrackedMarkdownHasNoCredentialShapedLiterals` now guards documentation prose, which is the one place the cookie and redaction rules do not look.
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

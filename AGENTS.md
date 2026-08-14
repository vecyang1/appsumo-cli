# AppSumo CLI Agent Notes

## Safety Boundary

This project is a read-only buyer-account CLI. Do not add commands that refund, transfer, activate, change plans, check out, or mutate AppSumo account data unless a future spec explicitly re-opens that design with confirmation gates.

## Secrets

Never commit or print:

- AppSumo cookies
- CSRF/session tokens
- account emails
- billing fields
- customer IDs
- license keys/codes
- raw CSV exports
- unredacted HAR files

Use ignored `tmp/` files for local auth handoff during smoke tests, and delete them after use.

Only send AppSumo cookies to `https://appsumo.com` or loopback test hosts. Do not make base URL overrides capable of forwarding real cookies to arbitrary hosts.

## Public Surfaces

Three surfaces are public and must never carry the buyer's session cookie:

| Surface | Endpoint | Command |
|---|---|---|
| Reviews | `GET /api/v2/deals/{deal_id}/reviews/` | `appsumo reviews` |
| Questions | `GET /api/v2/deals/{deal_id}/questions/` | `appsumo questions` |
| Catalog | `GET /api/v2/deals/esbrowse/` | `appsumo deals` |

Code reaching them must route through `Client.public()`, which strips the cookie, and the
commands must not consult any cookie source at all — `runtime.publicClient()` exists so that
this is structural rather than a convention. Attaching buyer credentials to a public crawl
leaks identity and changes per-user fields in the response.

### Every one of these endpoints hides a parameter that lies

Each has been measured live; do not re-derive by reading the site's own pager.

- **Reviews and questions accept `page` and ignore it.** `from` is the real offset. Covered by
  `TestFetchAllReviewsStopsWhenOffsetIsIgnored` and
  `TestFetchAllQuestionsInheritsTheOffsetGuard`. See `docs/03_reviews_api_discovery.md`.
- **The catalog needs `sort` or it loses rows.** `page` works, but only with a `sort` present:
  without it a full walk returned 305 of 363 declared deals, with 58 duplicates and no error.
  The sort *value* is ignored. Covered by `TestFetchAllDealsAlwaysSendsSort`.
- **`search_after` on catalog rows is not a usable cursor.** Sending it returns page one.

Any new pagination code needs a test that fails when the parameter is ignored, and a
reconciliation against a count the endpoint did not produce itself.

### Fields that are populated and meaningless

Reading these naively produces a specific, plausible, wrong answer, which is worse than a
crash because the user acts on it. Measured across the full live surface:

| Field | Where | Live value | Use instead |
|---|---|---|---|
| `is_redeemed` | account products | `false` on 70 of 70, including all 36 activated | `redeem_date` — see `Product.Redemption()` |
| `percent_claimed` | catalog | `-1` on 363 of 363 | nothing; report unknown |
| `codes_remaining` | catalog | `0` on 152 deals, all with `uses_codes: false` | only meaningful when `uses_codes` is true |
| `has_ended` | catalog | `false` on 363 of 363 | disappearing from the catalog |

The convention throughout is that a `*int` / `*bool` staying nil means "the API did not state
a value". Do not collapse those to zero anywhere, including in SQLite columns — the store has
a round-trip test for exactly that (`TestDealSnapshotRoundTripKeepsUnknownsUnknown`).

Details in `docs/04_catalog_and_questions_discovery.md`.

## Printing Press

Use CLI Printing Press as the generator path for the discovered read-only API contract. Keep the generated baseline under ignored `generated/` and keep hand-authored safety wrappers in the root module.

Regenerate with a clean output directory instead of relying on `--force`; a forced overwrite can preserve stale generated files and break verification:

```bash
rm -rf generated/appsumo-account-pp-cli
cli-printing-press generate --spec docs/openapi/appsumo-account.openapi.yaml --name appsumo-account --output generated/appsumo-account-pp-cli --spec-source browser-sniffed --transport browser-http --json
```

Do not force-add `generated/`. It is a local wheel-check artifact and may include raw session/product outputs that bypass the root CLI's redaction rules.

## Verification

Before claiming completion, run:

```bash
go test ./...
go vet ./...
./scripts/install_smoke.sh
rm -rf generated/appsumo-account-pp-cli
cli-printing-press generate --spec docs/openapi/appsumo-account.openapi.yaml --name appsumo-account --output generated/appsumo-account-pp-cli --spec-source browser-sniffed --transport browser-http --json
cli-printing-press shipcheck --dir generated/appsumo-account-pp-cli --spec docs/openapi/appsumo-account.openapi.yaml --no-live-check --json
```

Then run a tracked-file secret scan. Live smoke tests must use the logged-in browser session without printing cookies.

`TestTrackedMarkdownHasNoCredentialShapedLiterals` scans every git-known Markdown file for
credential-shaped example values, because prose is the one place the cookie and redaction
rules above do not look — a real Chrome Safe Storage key reached a release commit that way.
When it fires, replace the value with an obvious placeholder rather than widening the
allowlist.

Secret scans report verdicts, not bodies: use `grep -rl` or `grep -rc`, never `grep -n`, when
the pattern is a secret's name. `-n` prints the matching line, which puts the value into the
terminal and the session log — the check succeeds and the exposure happens anyway.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **26.05.23-appsumo-cli** (379 symbols, 905 relationships, 33 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/26.05.23-appsumo-cli/context` | Codebase overview, check index freshness |
| `gitnexus://repo/26.05.23-appsumo-cli/clusters` | All functional areas |
| `gitnexus://repo/26.05.23-appsumo-cli/processes` | All execution flows |
| `gitnexus://repo/26.05.23-appsumo-cli/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |
| Work in the Appsumo area (20 symbols) | `.claude/skills/generated/appsumo/SKILL.md` |
| Work in the Store area (16 symbols) | `.claude/skills/generated/store/SKILL.md` |
| Work in the Cli area (15 symbols) | `.claude/skills/generated/cli/SKILL.md` |
| Work in the Redact area (7 symbols) | `.claude/skills/generated/redact/SKILL.md` |

<!-- gitnexus:end -->

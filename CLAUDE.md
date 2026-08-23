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

# Public Surfaces

`appsumo reviews`, `appsumo questions`, and `appsumo deals` read **public** endpoints and
must never carry the session cookie. They route through `Client.public()` and
`runtime.publicClient()`, neither of which consults any cookie source.

Before touching pagination on any of them, read
[docs/03_reviews_api_discovery.md](docs/03_reviews_api_discovery.md) and
[docs/04_catalog_and_questions_discovery.md](docs/04_catalog_and_questions_discovery.md).
Each endpoint hides a parameter that lies, and all three failures are silent — HTTP 200 with
a healthy-looking `meta` on every request:

- Reviews and questions accept `page` and ignore it; `from` is the real offset. Guards:
  `TestFetchAllReviewsStopsWhenOffsetIsIgnored`, `TestFetchAllQuestionsInheritsTheOffsetGuard`.
- The catalog needs a `sort` present or a full walk returns 305 of 363 declared deals. The
  sort *value* is ignored. Guard: `TestFetchAllDealsAlwaysSendsSort`.
- `/products/<slug>/reviews/?page=N` is statically generated and serves the same five reviews
  at every page number.

# Fields That Are Populated And Wrong

`is_redeemed` is `false` on all 70 products of a live account including the 36 activated ones;
`percent_claimed` is `-1` on all 363 catalog deals; `codes_remaining` is `0` on 152 deals that
do not sell codes; `has_ended` is `false` on every deal the catalog serves. Use `redeem_date`,
report the rest as unknown, and keep `*int`/`*bool` nil rather than zero — including through
SQLite. See `AGENTS.md` for the full table.

# Releasing

The release process lives in [AGENTS.md](AGENTS.md) under **Releasing** — a
pointer rather than a copy, so there is one source of truth. Two things it says
that are easy to get wrong: release notes are tracked in `docs/releases/<tag>.md`
and published from there so the doc gates cover them, and the tag goes on the
commit CI passed on rather than on `main`, which can move in between.

# Public catalog and Q&A discovery

Verified live on 2026-08-14 with no credentials. Everything here is public: none
of these endpoints may carry the buyer's session cookie.

Companion to [03_reviews_api_discovery.md](03_reviews_api_discovery.md), which
covers the reviews surface and the decoys it hides behind.

## For agents

If you were asked to read AppSumo's public data, use these commands instead of
scraping HTML:

```bash
appsumo deals list --json
```

```bash
appsumo questions <product-slug> --json
```

Both reconcile themselves against the server's own count and report
`complete: true` / `false` / `null`. `null` means the server stated no total, so
completeness is **unknown** — not incomplete, and not complete. Warnings go to
stderr as well as into the JSON body, so a short crawl is visible whether you are
a human or a pipe.

To track the catalog over time:

```bash
appsumo deals sync
```

```bash
appsumo deals diff
```

## Endpoints

| Endpoint | Auth | Pagination | Envelope |
|---|---|---|---|
| `/api/v2/deals/esbrowse/` | none | `page` + `per_page`, **`sort` required** | `{deals: [...], meta: {total_results, total_pages, page, per_page}}` |
| `/api/v2/deals/{deal_id}/questions/` | none | `from` + `items_per_page` | `{comments: [...], meta: {total, count}}` |

Slug to `deal_id` resolution is unchanged from the reviews surface: read
`props.pageProps.deal.id` out of the `__NEXT_DATA__` island on
`/products/<slug>/reviews/`.

## The catalog needs a sort parameter or it loses rows

`page` genuinely works on `esbrowse`, unlike on the comment endpoints. But the
backing Elasticsearch query has no stable tiebreak unless a `sort` is supplied,
so pages overlap and rows fall through the gaps between them.

Walking `per_page=100` to exhaustion, three times in the same minute:

| Parameters | Unique deals | Declared | Duplicates | Result |
|---|---|---|---|---|
| `page` only | 305 | 363 | 58 | **58 lost** |
| `page` + `sort=newest` | 363 | 363 | 0 | complete |
| `page` + `sort=price` | 363 | 363 | 0 | complete |

Two things make this expensive to notice:

- **Every request returns HTTP 200** with a healthy-looking `meta` block. The
  broken walk's own `total_results` still says 363; only counting the unique
  slugs you actually received reveals the gap.
- **The `sort` value is ignored.** `newest` and `price` return the identical set
  in the identical order. So the parameter looks decorative under the obvious
  test — change the value, get the same answer — while its *presence* is what
  makes the walk complete. This is the mirror of the `page` decoy on the reviews
  endpoint: there, changing the value changed nothing because the parameter did
  nothing; here, changing the value changes nothing because only its existence
  matters.

`FetchAllDeals` therefore substitutes `DefaultDealsSort` for an empty sort rather
than forwarding it, and `TestFetchAllDealsAlwaysSendsSort` is the guard.

### `search_after` is a third decoy

Every catalog row carries a `search_after` array — the Elasticsearch cursor.
Sending it back as a query parameter is accepted and ignored: the response is
page one again. Do not build cursor pagination on it.

## Three catalog fields that are populated and meaningless

Measured across all 363 live deals:

| Field | Live value | Why a naive read is wrong |
|---|---|---|
| `percent_claimed` | `-1` on **363 of 363** | A sentinel, not a percentage. `max(0, …)` renders it as "0% claimed" on every deal. |
| `codes_remaining` | `0` on 152 deals, **all of which have `uses_codes: false`** | No deal that sells codes reports 0. `codes_remaining == 0 → sold out` is wrong 152 times out of 152. |
| `uses_limited_licensing` | `false` on **363 of 363** | The `limited_licenses_*` pair carries no information. |

`appsumo.Deal` exposes `codes_remaining` and `percent_claimed` as pointers that
stay `nil` unless the row proves the number means something, and the text output
prints nothing rather than `0 codes left`. The store keeps those columns
nullable for the same reason — writing `nil` as `0` would restore the confident
wrong number after one round trip.

## The catalog cannot tell you a deal ended

`has_ended` is `false` and `browse_deal_status` is `"current"` on all 363 rows.
This endpoint only ever serves live deals, so neither field can ever say
otherwise — a check reading them is a constant wearing a predicate's name.

`appsumo deals diff` therefore reports a slug that left the catalog as **`gone`**,
not `ended`, because leaving is the only evidence available and it cannot
distinguish sold out from expired from delisted.

That is also why `deals sync` **refuses to record an incomplete walk**. A
snapshot missing 58 deals would make the next diff report 58 healthy deals as
gone.

### What does move

`dates.end_date` is populated on 66 of 363 deals, with a `timer_reason` such as
`Price increase`. Together with `price` and `codes_remaining` those are the
fields `deals diff` actually compares.

## Questions

Same envelope and same offset contract as reviews, so the crawl is shared code
(`internal/appsumo/threads.go`) and a guard added for one surface protects the
other. Growify: **350 of 350** question threads in 5 requests.

Two differences from reviews, both visible in live data:

- **No rating.** A Q&A row has `resolved`, `pinned`, and `answer_type` where a
  review has `rating` and `would_recommend`. Stored rows keep `rating` NULL for
  questions and for replies; writing `0` would invent one-star rows.
- **`deal_id` is not the deal you asked for.** Growify's current deal is
  `257731`, and its oldest questions report `deal_id: 213645` — Q&A carries
  across a product's earlier deal runs. Do not assert equality.

### "Answered" is a property of the thread, not a field

`resolved` is set on the answering *reply*, not on the question, and is `null` on
plenty of answered threads. `Question.Answered()` reports whether the thread has
any replies at all.

Because growify returns 350 of 350 answered, that predicate looks like it can
never say no. It can — confirmed against other products the same day:

| Product | Threads | Unanswered |
|---|---|---|
| growify | 350 | 0 |
| fuseai | 30 | 0 |
| postbeam | 14 | **1** |
| aiwritebook | 48 | **2** |

## Verification

| Command | Result | Independent check |
|---|---|---|
| `appsumo deals list` | 363 of 363 in 4 requests | matches `meta.total_results`, and the unsorted walk's 305 proves the count is not self-fulfilling |
| `appsumo questions growify` | 350 of 350 in 5 requests, 350 answered | matches `meta.total`; per-thread reply counts range 1–6 |
| `appsumo questions postbeam` | 14 of 14, 13 answered | the unanswered thread is visible on the product page |
| `appsumo portfolio` | 68 redeemed of 70 | the 2 exceptions are AppSumo Plus subscription rows, which carry no `redeem_date` by design |

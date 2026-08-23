package appsumo

// Public deal catalog.
//
//	GET /api/v2/deals/esbrowse/?page=1&per_page=100&sort=newest
//
// This is the browse surface behind appsumo.com's own listing pages. It is
// public and must never carry the buyer's session cookie.
//
// # Pagination requires a sort parameter
//
// `page` works here, unlike on the comment endpoints — but only when a `sort` is
// also sent. Without one the backing Elasticsearch query has no stable tiebreak,
// so pages overlap and rows fall through the gaps. Measured 2026-08-14 against
// the live catalog, walking per_page=100 to exhaustion:
//
//	page only, no sort   305 unique of 363 declared, 58 duplicates, 58 LOST
//	page + sort=newest   363 unique of 363 declared, 0 duplicates
//	page + sort=price    363 unique of 363 declared, 0 duplicates
//
// The sort *value* is ignored — newest and price return the identical set in the
// identical order — but its *presence* is what makes the walk complete. Both
// walks return HTTP 200 with a plausible-looking meta on every page, so the only
// evidence of the broken one is reconciling unique rows against total_results.
// DefaultDealsSort is therefore not a preference; removing it loses 16% of the
// catalog silently.
//
// `search_after` appears on every row and is not accepted as a query parameter:
// sending it returns page one again. It is a decoy, like `page` on the comment
// endpoints.
//
// # Three fields that are populated but meaningless
//
// Measured across all 363 live deals on 2026-08-14:
//
//   - percent_claimed is -1 on 363 of 363. It is a sentinel, not a percentage.
//   - codes_remaining is 0 on 152 deals, every one of which has uses_codes
//     false. No deal that uses codes reports 0. So a bare `codes_remaining == 0`
//     reads as "sold out" and is wrong every single time it fires.
//   - uses_limited_licensing is false on 363 of 363, so the limited_licenses_*
//     pair carries no information.
//
// Deal therefore exposes these as pointers that stay nil unless the row proves
// the value means something. Absent is not zero.
//
// # This endpoint only ever returns live deals
//
// has_ended is false and browse_deal_status is "current" on all 363 rows, so
// neither field can report that a deal ended. Disappearing from the catalog is
// the only available ended signal, which is why DiffDeals treats a dropped slug
// as "gone" rather than looking for a status flag that never changes.

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	// DefaultDealsPageSize is the browse window size. The API honours up to at
	// least 500, but 100 keeps any one response small.
	DefaultDealsPageSize = 100

	// DefaultDealsSort is required for a complete walk; see the note above.
	DefaultDealsSort = "newest"

	maxDealsRequests = 60
)

// DealsMeta is the browse envelope's own count of the catalog.
//
// Pointers keep an absent field unknown rather than a confident zero.
type DealsMeta struct {
	TotalResults *int `json:"total_results"`
	TotalPages   *int `json:"total_pages"`
	Page         *int `json:"page"`
	PerPage      *int `json:"per_page"`
}

type dealsEnvelope struct {
	Deals []json.RawMessage `json:"deals"`
	Meta  DealsMeta         `json:"meta"`
}

// dealPayload is the wire shape. Deal is the normalised view callers see.
type dealPayload struct {
	ID            flexInt64   `json:"id"`
	Slug          string      `json:"slug"`
	PublicName    string      `json:"public_name"`
	Price         flexFloat64 `json:"price"`
	OriginalPrice flexFloat64 `json:"original_price"`
	PlusPrice     flexFloat64 `json:"plus_discount_price"`
	IsFree        bool        `json:"is_free"`
	ListingType   string      `json:"listing_type"`
	AbsoluteURL   string      `json:"get_absolute_url"`

	UsesCodes                bool `json:"uses_codes"`
	CodesRemaining           *int `json:"codes_remaining"`
	UsesLimitedLicensing     bool `json:"uses_limited_licensing"`
	LimitedLicensesRemaining *int `json:"limited_licenses_remaining"`
	PercentClaimed           *int `json:"percent_claimed"`

	Dates struct {
		StartDate   string `json:"start_date"`
		EndDate     string `json:"end_date"`
		TimerReason *struct {
			Reason string `json:"reason"`
		} `json:"timer_reason"`
	} `json:"dates"`

	DealReview *struct {
		ReviewCount   *int         `json:"review_count"`
		AverageRating *flexFloat64 `json:"average_rating"`
	} `json:"deal_review"`

	Taxonomy map[string]struct {
		ValueEnumeration string `json:"value_enumeration"`
	} `json:"taxonomy"`
}

// Deal is one catalog listing, normalised so that a nil field means the API did
// not state a value rather than that the value is zero.
type Deal struct {
	ID            int64   `json:"id"`
	Slug          string  `json:"slug"`
	Name          string  `json:"name"`
	URL           string  `json:"url"`
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"original_price"`
	PlusPrice     float64 `json:"plus_price"`
	IsFree        bool    `json:"is_free"`
	ListingType   string  `json:"listing_type"`

	// CodesRemaining is nil unless the deal actually sells codes. See the
	// package note: a bare zero here means "not applicable" in live data.
	CodesRemaining *int `json:"codes_remaining"`
	// PercentClaimed is nil whenever the API sends its -1 sentinel, which as of
	// 2026-08-14 is every deal.
	PercentClaimed *int `json:"percent_claimed"`

	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	TimerReason string `json:"timer_reason"`

	ReviewCount   *int     `json:"review_count"`
	AverageRating *float64 `json:"average_rating"`

	Category string `json:"category"`
	Group    string `json:"group"`

	// Raw is the unmodified catalog row, kept so a later agent can read a field
	// this struct does not model yet.
	Raw json.RawMessage `json:"-"`
}

// DealsResult is a full catalog walk plus everything needed to judge it.
//
// Sort and PageSize are what the walk actually sent, not what the caller asked
// for. On this endpoint that distinction decides whether the result is
// trustworthy: a blank sort is substituted, and a report echoing the blank would
// label a complete walk with the setting that loses 16% of the catalog.
type DealsResult struct {
	Deals         []Deal
	DeclaredTotal *int
	Requests      int
	Truncated     bool
	Warnings      []string
	Sort          string
	PageSize      int
}

// Complete reports whether the walk collected exactly as many deals as the
// catalog declared. It is nil when the catalog declared no total, because an
// unverifiable crawl is unknown — neither complete nor incomplete.
func (r *DealsResult) Complete() *bool {
	if r.DeclaredTotal == nil {
		return nil
	}
	complete := !r.Truncated && len(r.Deals) == *r.DeclaredTotal
	return &complete
}

func normaliseDeal(payload dealPayload, raw json.RawMessage) Deal {
	deal := Deal{
		ID:            int64(payload.ID),
		Slug:          payload.Slug,
		Name:          payload.PublicName,
		Price:         float64(payload.Price),
		OriginalPrice: float64(payload.OriginalPrice),
		PlusPrice:     float64(payload.PlusPrice),
		IsFree:        payload.IsFree,
		ListingType:   payload.ListingType,
		StartDate:     payload.Dates.StartDate,
		EndDate:       payload.Dates.EndDate,
		Raw:           raw,
	}
	if payload.AbsoluteURL != "" {
		deal.URL = payload.AbsoluteURL
	} else if payload.Slug != "" {
		deal.URL = "/products/" + payload.Slug + "/"
	}
	if reason := payload.Dates.TimerReason; reason != nil {
		deal.TimerReason = reason.Reason
	}

	// Stock counters are only meaningful when the deal says it uses them.
	if payload.UsesCodes && payload.CodesRemaining != nil {
		remaining := *payload.CodesRemaining
		deal.CodesRemaining = &remaining
	}
	if payload.PercentClaimed != nil && *payload.PercentClaimed >= 0 {
		claimed := *payload.PercentClaimed
		deal.PercentClaimed = &claimed
	}

	if review := payload.DealReview; review != nil {
		if review.ReviewCount != nil {
			count := *review.ReviewCount
			deal.ReviewCount = &count
		}
		if review.AverageRating != nil {
			rating := float64(*review.AverageRating)
			deal.AverageRating = &rating
		}
	}
	if node, ok := payload.Taxonomy["category"]; ok {
		deal.Category = node.ValueEnumeration
	}
	if node, ok := payload.Taxonomy["group"]; ok {
		deal.Group = node.ValueEnumeration
	}
	return deal
}

// FetchDealsPage reads one browse page of the public catalog.
func (c *Client) FetchDealsPage(ctx context.Context, page, perPage int, sort string) ([]Deal, DealsMeta, error) {
	if page < 1 {
		page = 1
	}
	if perPage <= 0 {
		perPage = DefaultDealsPageSize
	}
	// An empty sort is not a neutral default here: it is the setting that loses
	// rows. Substitute rather than forward it.
	sort = firstNonBlank(sort, DefaultDealsSort)

	var envelope dealsEnvelope
	err := c.public().getJSON(ctx, "/api/v2/deals/esbrowse/", map[string]string{
		"page":     strconv.Itoa(page),
		"per_page": strconv.Itoa(perPage),
		"sort":     sort,
	}, &envelope)
	if err != nil {
		return nil, DealsMeta{}, err
	}

	deals := make([]Deal, 0, len(envelope.Deals))
	for index, raw := range envelope.Deals {
		var payload dealPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, envelope.Meta, fmt.Errorf("decode catalog row %d on page %d: %w", index, page, err)
		}
		deals = append(deals, normaliseDeal(payload, raw))
	}
	return deals, envelope.Meta, nil
}

// FetchAllDeals walks the whole catalog, deduplicating by slug and reconciling
// the result against the catalog's own declared total.
//
// limit caps the collected deals; 0 means no cap.
func (c *Client) FetchAllDeals(ctx context.Context, perPage int, sort string, limit int) (*DealsResult, error) {
	if perPage <= 0 {
		perPage = DefaultDealsPageSize
	}
	sort = firstNonBlank(sort, DefaultDealsSort)
	result := &DealsResult{Warnings: []string{}, Sort: sort, PageSize: perPage}
	seen := make(map[string]struct{})
	duplicates := 0

	// See fetchThread: a caller-requested cap is expected and must not warn about
	// its own short count, while an anomaly-triggered stop should report the
	// shortfall. Both mark the walk Truncated.
	cappedOnPurpose := false

	for page := 1; page <= maxDealsRequests; page++ {
		deals, meta, err := c.FetchDealsPage(ctx, page, perPage, sort)
		if err != nil {
			return nil, err
		}
		result.Requests++
		if result.DeclaredTotal == nil && meta.TotalResults != nil {
			total := *meta.TotalResults
			result.DeclaredTotal = &total
		}
		if len(deals) == 0 {
			break
		}

		added := 0
		for _, deal := range deals {
			key := deal.Slug
			if key == "" {
				key = strconv.FormatInt(deal.ID, 10)
			}
			if _, duplicate := seen[key]; duplicate {
				duplicates++
				continue
			}
			seen[key] = struct{}{}
			result.Deals = append(result.Deals, deal)
			added++
			if limit > 0 && len(result.Deals) >= limit {
				break
			}
		}

		if limit > 0 && len(result.Deals) >= limit {
			result.Truncated = true
			cappedOnPurpose = true
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"stopped at --limit %d; more deals exist", limit))
			break
		}
		// A page of slugs we already hold means the ordering is not stable and
		// paging further would keep re-reading the same window.
		//
		// Truncated is set here for the same reason it is set on the other two
		// early exits: the walk stopped for a reason other than exhausting the
		// catalog. Without it, Complete() would fall back to comparing the
		// collected count against a total supplied by the very response stream
		// that just proved it re-serves windows — and a coincidental match would
		// report a walk that hit a known anomaly as verified. `deals sync` then
		// persists it, and the next diff calls the missing deals gone.
		if added == 0 {
			result.Truncated = true
			result.Warnings = append(result.Warnings, fmt.Sprintf(
				"page %d returned %d deals but no new slugs; the catalog ordering is unstable — walk stopped early",
				page, len(deals)))
			break
		}
		if len(deals) < perPage {
			break
		}
	}

	if result.Requests >= maxDealsRequests {
		result.Truncated = true
		cappedOnPurpose = true
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"stopped after the %d request safety cap", maxDealsRequests))
	}
	if duplicates > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"catalog served %d duplicate rows across pages; deduplicated by slug", duplicates))
	}
	switch {
	case result.DeclaredTotal == nil:
		result.Warnings = append(result.Warnings,
			"catalog carried no meta.total_results; completeness could not be verified")
	case !cappedOnPurpose && len(result.Deals) != *result.DeclaredTotal:
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"collected %d deals but the catalog declared %d; %d rows were never served",
			len(result.Deals), *result.DeclaredTotal, *result.DeclaredTotal-len(result.Deals)))
	}
	return result, nil
}

// DealChange is one difference between two catalog snapshots.
type DealChange struct {
	Kind string `json:"kind"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	URL  string `json:"url"`
	// Field, Before, and After are set on "changed" rows only.
	Field  string `json:"field,omitempty"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

// DiffDeals reports what moved between an earlier and a later catalog snapshot.
//
// A slug present before and absent after is reported as "gone" rather than
// "ended": this endpoint never marks a deal ended (has_ended is false on every
// row it serves), so leaving the catalog is the only evidence available, and it
// cannot distinguish sold out from expired from delisted.
func DiffDeals(before, after []Deal) []DealChange {
	previous := indexBySlug(before)
	current := indexBySlug(after)

	var changes []DealChange
	for _, deal := range after {
		old, existed := previous[deal.Slug]
		if !existed {
			changes = append(changes, DealChange{Kind: "new", Slug: deal.Slug, Name: deal.Name, URL: deal.URL})
			continue
		}
		changes = append(changes, comparePricesAndStock(old, deal)...)
	}
	for _, deal := range before {
		if _, stillListed := current[deal.Slug]; !stillListed {
			changes = append(changes, DealChange{Kind: "gone", Slug: deal.Slug, Name: deal.Name, URL: deal.URL})
		}
	}
	return changes
}

func comparePricesAndStock(before, after Deal) []DealChange {
	var changes []DealChange
	record := func(field, oldValue, newValue string) {
		if oldValue == newValue {
			return
		}
		changes = append(changes, DealChange{
			Kind: "changed", Slug: after.Slug, Name: after.Name, URL: after.URL,
			Field: field, Before: oldValue, After: newValue,
		})
	}
	record("price", money(before.Price), money(after.Price))
	record("original_price", money(before.OriginalPrice), money(after.OriginalPrice))
	// Unknown stays "unknown" on both sides, so a field that was never readable
	// cannot masquerade as a change.
	record("codes_remaining", optionalInt(before.CodesRemaining), optionalInt(after.CodesRemaining))
	record("end_date", before.EndDate, after.EndDate)
	record("timer_reason", before.TimerReason, after.TimerReason)
	return changes
}

func indexBySlug(deals []Deal) map[string]Deal {
	index := make(map[string]Deal, len(deals))
	for _, deal := range deals {
		index[deal.Slug] = deal
	}
	return index
}

func money(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func optionalInt(value *int) string {
	if value == nil {
		return "unknown"
	}
	return strconv.Itoa(*value)
}

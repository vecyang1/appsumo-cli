package appsumo_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
)

// dealFixture mirrors a captured /api/v2/deals/esbrowse/ row, including the two
// sentinels the live catalog sends on every deal: percent_claimed -1, and a
// codes_remaining of 0 on a deal that does not sell codes.
func dealFixture(index int) map[string]any {
	return map[string]any{
		"id":                         260000 + index,
		"slug":                       fmt.Sprintf("deal-%d", index),
		"public_name":                fmt.Sprintf("Deal %d", index),
		"price":                      49.0,
		"original_price":             588.0,
		"plus_discount_price":        44.1,
		"is_free":                    false,
		"listing_type":               "marketplace",
		"get_absolute_url":           fmt.Sprintf("/products/deal-%d/", index),
		"uses_codes":                 false,
		"codes_remaining":            0,
		"uses_limited_licensing":     false,
		"limited_licenses_remaining": 0,
		"percent_claimed":            -1,
		"has_ended":                  false,
		"browse_deal_status":         "current",
		"dates": map[string]any{
			"start_date": "2026-08-10T12:50:27Z", "end_date": nil, "timer_reason": nil,
		},
		"deal_review": map[string]any{"review_count": 12, "average_rating": 4.68},
		"taxonomy": map[string]any{
			"category": map[string]any{"value_enumeration": "seo-analytics"},
			"group":    map[string]any{"value_enumeration": "marketing"},
		},
	}
}

// catalogServer pages a fixed catalog, and refuses any request that omits sort —
// which is what the live endpoint effectively does by silently losing rows.
func catalogServer(t *testing.T, total int, seenQueries *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			t.Errorf("catalog request carried a session cookie: %s", r.URL.Path)
		}
		if seenQueries != nil {
			*seenQueries = append(*seenQueries, r.URL.RawQuery)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if perPage <= 0 {
			perPage = 100
		}
		rows := []map[string]any{}
		for i := (page - 1) * perPage; i < page*perPage && i < total; i++ {
			rows = append(rows, dealFixture(i))
		}
		writeJSON(t, w, map[string]any{
			"deals": rows,
			"meta": map[string]any{
				"total_results": total, "total_pages": (total + perPage - 1) / perPage,
				"page": page, "per_page": perPage,
			},
		})
	}))
}

func TestFetchAllDealsWalksEveryPage(t *testing.T) {
	var queries []string
	server := catalogServer(t, 363, &queries)
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
	})

	result, err := client.FetchAllDeals(context.Background(), 100, appsumo.DefaultDealsSort, 0)
	if err != nil {
		t.Fatalf("FetchAllDeals returned error: %v", err)
	}
	if len(result.Deals) != 363 {
		t.Fatalf("collected %d deals, want 363", len(result.Deals))
	}
	if complete := result.Complete(); complete == nil || !*complete {
		t.Fatalf("complete = %v, want true", complete)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("healthy walk emitted warnings: %v", result.Warnings)
	}
	if len(queries) != 4 {
		t.Fatalf("made %d requests, want 4 (100+100+100+63)", len(queries))
	}
}

// TestFetchAllDealsAlwaysSendsSort pins the single parameter that decides
// whether the walk is complete. Measured against the live catalog on
// 2026-08-14: without sort the same walk returned 305 of 363 declared deals,
// with no error and a healthy-looking meta on every page.
func TestFetchAllDealsAlwaysSendsSort(t *testing.T) {
	for _, requested := range []string{"", "   ", "newest", "price"} {
		var queries []string
		server := catalogServer(t, 120, &queries)
		client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})

		if _, err := client.FetchAllDeals(context.Background(), 100, requested, 0); err != nil {
			server.Close()
			t.Fatalf("FetchAllDeals(%q) returned error: %v", requested, err)
		}
		server.Close()

		if len(queries) == 0 {
			t.Fatalf("FetchAllDeals(%q) made no requests", requested)
		}
		for _, query := range queries {
			values, err := url.ParseQuery(query)
			if err != nil {
				t.Fatalf("unparseable query %q: %v", query, err)
			}
			if values.Get("sort") == "" {
				t.Fatalf("FetchAllDeals(%q) sent a request with no sort: %s", requested, query)
			}
		}
	}
}

func TestFetchAllDealsWarnsWhenCatalogIsShort(t *testing.T) {
	// Declares 363 and only ever serves 305, exactly as the unsorted live walk did.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		rows := []map[string]any{}
		for i := (page - 1) * 100; i < page*100 && i < 305; i++ {
			rows = append(rows, dealFixture(i))
		}
		writeJSON(t, w, map[string]any{
			"deals": rows,
			"meta":  map[string]any{"total_results": 363, "total_pages": 4, "page": page, "per_page": 100},
		})
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllDeals(context.Background(), 100, "newest", 0)
	if err != nil {
		t.Fatalf("FetchAllDeals returned error: %v", err)
	}
	if complete := result.Complete(); complete == nil || *complete {
		t.Fatalf("a 305-of-363 walk reported complete = %v", complete)
	}
	if !hasWarningContaining(result.Warnings, "58 rows were never served") {
		t.Fatalf("short catalog was not reconciled: %v", result.Warnings)
	}
}

func TestFetchAllDealsStopsWhenOrderingIsUnstable(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		rows := []map[string]any{}
		for i := 0; i < 5; i++ {
			rows = append(rows, dealFixture(i))
		}
		writeJSON(t, w, map[string]any{
			"deals": rows,
			"meta":  map[string]any{"total_results": 363, "total_pages": 73, "page": 1, "per_page": 5},
		})
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllDeals(context.Background(), 5, "newest", 0)
	if err != nil {
		t.Fatalf("FetchAllDeals returned error: %v", err)
	}
	if len(result.Deals) != 5 {
		t.Fatalf("collected %d deals, want the single distinct window of 5", len(result.Deals))
	}
	if requests > 2 {
		t.Fatalf("kept paging a catalog that repeats page one: %d requests", requests)
	}
	if !hasWarningContaining(result.Warnings, "no new slugs") {
		t.Fatalf("unstable ordering was not announced: %v", result.Warnings)
	}
}

// TestFetchAllDealsKeepsSentinelsUnknown is the "absent is not zero" guard for
// this surface. A -1 percentage and a codes count on a deal that sells no codes
// are both non-answers, and rendering either as 0 states a fact the API never did.
func TestFetchAllDealsKeepsSentinelsUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		notApplicable := dealFixture(1)
		realZero := dealFixture(2)
		realZero["uses_codes"] = true
		realZero["codes_remaining"] = 0
		stocked := dealFixture(3)
		stocked["uses_codes"] = true
		stocked["codes_remaining"] = 1126
		stocked["percent_claimed"] = 42
		writeJSON(t, w, map[string]any{
			"deals": []map[string]any{notApplicable, realZero, stocked},
			"meta":  map[string]any{"total_results": 3, "total_pages": 1, "page": 1, "per_page": 100},
		})
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllDeals(context.Background(), 100, "newest", 0)
	if err != nil {
		t.Fatalf("FetchAllDeals returned error: %v", err)
	}
	if len(result.Deals) != 3 {
		t.Fatalf("collected %d deals, want 3", len(result.Deals))
	}

	notApplicable, realZero, stocked := result.Deals[0], result.Deals[1], result.Deals[2]
	if notApplicable.CodesRemaining != nil {
		t.Fatalf("a deal that sells no codes reported %d remaining", *notApplicable.CodesRemaining)
	}
	if realZero.CodesRemaining == nil || *realZero.CodesRemaining != 0 {
		t.Fatalf("a genuine sold-out zero was erased to unknown: %v", realZero.CodesRemaining)
	}
	if notApplicable.PercentClaimed != nil {
		t.Fatalf("the -1 sentinel became %d percent claimed", *notApplicable.PercentClaimed)
	}
	if stocked.PercentClaimed == nil || *stocked.PercentClaimed != 42 {
		t.Fatalf("a real percentage was dropped: %v", stocked.PercentClaimed)
	}
	if stocked.AverageRating == nil || *stocked.AverageRating != 4.68 {
		t.Fatalf("average rating not parsed: %v", stocked.AverageRating)
	}
	if stocked.Category != "seo-analytics" || stocked.Group != "marketing" {
		t.Fatalf("taxonomy not parsed: %#v", stocked)
	}
}

func TestDiffDealsReportsArrivalsDeparturesAndPriceMoves(t *testing.T) {
	before := []appsumo.Deal{
		{Slug: "stays", Name: "Stays", Price: 49},
		{Slug: "drops", Name: "Drops", Price: 99},
		{Slug: "vanishes", Name: "Vanishes", Price: 39},
	}
	after := []appsumo.Deal{
		{Slug: "stays", Name: "Stays", Price: 49},
		{Slug: "drops", Name: "Drops", Price: 69},
		{Slug: "arrives", Name: "Arrives", Price: 59},
	}

	changes := appsumo.DiffDeals(before, after)
	kinds := map[string][]appsumo.DealChange{}
	for _, change := range changes {
		kinds[change.Kind] = append(kinds[change.Kind], change)
	}

	if len(kinds["new"]) != 1 || kinds["new"][0].Slug != "arrives" {
		t.Fatalf("arrivals wrong: %#v", kinds["new"])
	}
	if len(kinds["gone"]) != 1 || kinds["gone"][0].Slug != "vanishes" {
		t.Fatalf("departures wrong: %#v", kinds["gone"])
	}
	if len(kinds["changed"]) != 1 {
		t.Fatalf("expected exactly one price move, got %#v", kinds["changed"])
	}
	move := kinds["changed"][0]
	if move.Slug != "drops" || move.Field != "price" || move.Before != "99.00" || move.After != "69.00" {
		t.Fatalf("price move wrong: %#v", move)
	}
}

// A field that was never readable must not read as a change when it is still
// unreadable — otherwise every diff invents movement on all 363 deals.
func TestDiffDealsDoesNotInventChangesFromUnknowns(t *testing.T) {
	unknownStock := appsumo.Deal{Slug: "quiet", Name: "Quiet", Price: 49}
	changes := appsumo.DiffDeals([]appsumo.Deal{unknownStock}, []appsumo.Deal{unknownStock})
	if len(changes) != 0 {
		t.Fatalf("identical snapshots produced changes: %#v", changes)
	}

	stocked := unknownStock
	remaining := 500
	stocked.CodesRemaining = &remaining
	appeared := appsumo.DiffDeals([]appsumo.Deal{unknownStock}, []appsumo.Deal{stocked})
	if len(appeared) != 1 || appeared[0].Before != "unknown" || appeared[0].After != "500" {
		t.Fatalf("unknown-to-known transition not reported cleanly: %#v", appeared)
	}
}

// TestFetchAllDealsWillNotCertifyAnAnomalousWalk is the case a code review
// constructed and the first implementation got wrong.
//
// The server declares 120 and serves all 120, then re-serves an already-seen
// window instead of an empty page — the unstable-ordering signature. The
// collected count matches the declared total, so a Complete() that trusted only
// the count would report the walk verified despite the walk having hit a known
// anomaly. `deals sync` reads exactly that verdict to decide whether to persist
// a snapshot, and a bad snapshot makes the next diff call healthy deals gone.
//
// The declared total cannot rescue this: it comes from the same response stream
// that just proved it re-serves windows.
func TestFetchAllDealsWillNotCertifyAnAnomalousWalk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		// 200 rows over two full pages, so the walk does not exit through the
		// short-page break and actually reaches the third request.
		start := (page - 1) * 100
		if start >= 200 {
			// Instead of an empty page, hand back page one again.
			start = 0
		}
		rows := []map[string]any{}
		for i := start; i < start+100 && i < 200; i++ {
			rows = append(rows, dealFixture(i))
		}
		writeJSON(t, w, map[string]any{
			"deals": rows,
			"meta":  map[string]any{"total_results": 200, "total_pages": 2, "page": page, "per_page": 100},
		})
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllDeals(context.Background(), 100, "newest", 0)
	if err != nil {
		t.Fatalf("FetchAllDeals returned error: %v", err)
	}
	if len(result.Deals) != 200 || result.DeclaredTotal == nil || *result.DeclaredTotal != 200 {
		t.Fatalf("precondition wrong: collected %d of %v", len(result.Deals), result.DeclaredTotal)
	}
	if !result.Truncated {
		t.Fatal("a walk that stopped on unstable ordering was not marked truncated")
	}
	if complete := result.Complete(); complete == nil || *complete {
		t.Fatalf("an anomalous walk certified itself complete = %v", complete)
	}
	if !hasWarningContaining(result.Warnings, "no new slugs") {
		t.Fatalf("the anomaly was not announced: %v", result.Warnings)
	}
}

// A --limit stop is expected, so it must not also warn that its count
// disagrees with the total. Otherwise every capped run emits a scary warning
// and readers learn to skip all of them.
func TestFetchAllDealsLimitDoesNotWarnAboutItsOwnShortCount(t *testing.T) {
	server := catalogServer(t, 363, nil)
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllDeals(context.Background(), 100, "newest", 10)
	if err != nil {
		t.Fatalf("FetchAllDeals returned error: %v", err)
	}
	if !result.Truncated {
		t.Fatal("a --limit run was not marked truncated")
	}
	if hasWarningContaining(result.Warnings, "never served") {
		t.Fatalf("a deliberate --limit stop warned about missing rows: %v", result.Warnings)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected exactly the --limit warning, got %v", result.Warnings)
	}
}

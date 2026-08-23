package appsumo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
)

// reviewFixture mirrors a captured /api/v2/deals/{id}/reviews/ record: numeric
// ids, a null rating on a reply, and a nested founder response.
func reviewFixture(id int) map[string]any {
	return map[string]any{
		"id": id, "deal_id": 257731, "parent_id": nil, "level": 0,
		"title":   fmt.Sprintf("Review %d", id),
		"comment": fmt.Sprintf("Body of review %d", id),
		"rating":  5, "created": "2025-01-01T00:00:00Z", "modified": "2025-01-02T00:00:00Z",
		"up_votes": 2, "down_votes": 0, "would_recommend": nil, "incentivized": false,
		"purchased": true, "approved": true, "edited": false, "status": "Approved",
		"display_path": fmt.Sprintf("/products/fixture/reviews/review-%d/", id),
		"user": map[string]any{
			"id": 1000 + id, "username": fmt.Sprintf("user%d", id),
			"date_joined": "2016-04-16T07:06:11Z", "deals_purchased": 17,
		},
		"children": []map[string]any{{
			"id": 900000 + id, "deal_id": 257731, "parent_id": id, "level": 1,
			"comment": "Thanks for the feedback!", "rating": nil,
			"created": "2025-01-03T00:00:00Z",
			"user":    map[string]any{"id": 42, "username": "founder"},
		}},
	}
}

// offsetServer honours `from`, the way the live API does.
func offsetServer(t *testing.T, total int, seenQueries *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			t.Errorf("review request carried a session cookie: %s", r.URL.Path)
		}
		if seenQueries != nil {
			*seenQueries = append(*seenQueries, r.URL.RawQuery)
		}
		from, _ := strconv.Atoi(r.URL.Query().Get("from"))
		size, _ := strconv.Atoi(r.URL.Query().Get("items_per_page"))
		if size <= 0 {
			size = 20
		}
		comments := []map[string]any{}
		for i := from; i < from+size && i < total; i++ {
			comments = append(comments, reviewFixture(1000+i))
		}
		writeJSON(t, w, map[string]any{
			"comments": comments,
			"meta":     map[string]any{"total": total, "count": len(comments)},
		})
	}))
}

func TestFetchAllReviewsWalksEveryOffset(t *testing.T) {
	var queries []string
	server := offsetServer(t, 224, &queries)
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
	})

	result, err := client.FetchAllReviews(context.Background(), appsumo.ReviewsQuery{DealID: 257731, PageSize: 100}, 0)
	if err != nil {
		t.Fatalf("FetchAllReviews returned error: %v", err)
	}
	if len(result.Reviews) != 224 {
		t.Fatalf("collected %d reviews, want 224", len(result.Reviews))
	}
	if result.ExpectedTotal == nil || *result.ExpectedTotal != 224 {
		t.Fatalf("expected total = %v, want 224", result.ExpectedTotal)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("healthy crawl emitted warnings: %v", result.Warnings)
	}
	if result.Reviews[0].Children == nil || result.Reviews[0].Children[0].User.Username != "founder" {
		t.Fatalf("nested reply was dropped: %#v", result.Reviews[0].Children)
	}
	for _, query := range queries {
		if !strings.Contains(query, "from=") || !strings.Contains(query, "items_per_page=") {
			t.Fatalf("request missing offset parameters: %s", query)
		}
		// The live API accepts `page` and ignores it. Sending it would make a
		// broken crawl look intentional.
		if strings.Contains(query, "page=") && !strings.Contains(query, "items_per_page=") {
			t.Fatalf("request used the decoy page parameter: %s", query)
		}
	}
}

// TestFetchAllReviewsStopsWhenOffsetIsIgnored pins the failure mode that the
// live site actually has: a paginated endpoint that returns page one forever.
// A crawler without the duplicate guard would spin to its request cap and
// report 100 copies of the same window as a full result set.
func TestFetchAllReviewsStopsWhenOffsetIsIgnored(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		comments := []map[string]any{}
		for i := 0; i < 5; i++ {
			comments = append(comments, reviewFixture(2000+i))
		}
		writeJSON(t, w, map[string]any{
			"comments": comments,
			"meta":     map[string]any{"total": 224, "count": len(comments)},
		})
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllReviews(context.Background(), appsumo.ReviewsQuery{DealID: 1, PageSize: 5}, 0)
	if err != nil {
		t.Fatalf("FetchAllReviews returned error: %v", err)
	}
	if len(result.Reviews) != 5 {
		t.Fatalf("collected %d reviews, want the single distinct window of 5", len(result.Reviews))
	}
	if requests > 2 {
		t.Fatalf("kept requesting an endpoint that ignores `from`: %d requests", requests)
	}
	if !hasWarningContaining(result.Warnings, "ignore `from`") {
		t.Fatalf("no warning about the ignored offset: %v", result.Warnings)
	}
	if !hasWarningContaining(result.Warnings, "meta.total reported 224") {
		t.Fatalf("short crawl was not reconciled against meta.total: %v", result.Warnings)
	}
}

func TestFetchAllReviewsReportsUnknownTotal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("from") == "0" {
			writeJSON(t, w, map[string]any{"comments": []map[string]any{reviewFixture(1)}, "meta": map[string]any{}})
			return
		}
		writeJSON(t, w, map[string]any{"comments": []map[string]any{}, "meta": map[string]any{}})
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllReviews(context.Background(), appsumo.ReviewsQuery{DealID: 1, PageSize: 1}, 0)
	if err != nil {
		t.Fatalf("FetchAllReviews returned error: %v", err)
	}
	if result.ExpectedTotal != nil {
		t.Fatalf("absent meta.total became %v, want unknown", *result.ExpectedTotal)
	}
	if !hasWarningContaining(result.Warnings, "completeness could not be verified") {
		t.Fatalf("missing unknown-total warning: %v", result.Warnings)
	}
}

func TestFetchAllReviewsHonoursLimit(t *testing.T) {
	server := offsetServer(t, 224, nil)
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllReviews(context.Background(), appsumo.ReviewsQuery{DealID: 1, PageSize: 100}, 10)
	if err != nil {
		t.Fatalf("FetchAllReviews returned error: %v", err)
	}
	if len(result.Reviews) != 10 {
		t.Fatalf("collected %d reviews, want 10", len(result.Reviews))
	}
	if !result.Truncated || !hasWarningContaining(result.Warnings, "--limit 10") {
		t.Fatalf("truncation was not announced: truncated=%v warnings=%v", result.Truncated, result.Warnings)
	}
}

func TestResolveProductReadsNextDataWithoutCookie(t *testing.T) {
	var seenCookie string
	var seenPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCookie = r.Header.Get("Cookie")
		seenPath = r.URL.Path
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, nextDataPage(t))
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
	})

	product, err := client.ResolveProduct(context.Background(), "growify")
	if err != nil {
		t.Fatalf("ResolveProduct returned error: %v", err)
	}
	if seenCookie != "" {
		t.Fatalf("public product page request carried a session cookie: %q", seenCookie)
	}
	if seenPath != "/products/growify/reviews/" {
		t.Fatalf("unexpected path %s", seenPath)
	}
	if product.DealID != 257731 || product.Name != "Growify" {
		t.Fatalf("unexpected product: %#v", product)
	}
	if product.Ratings == nil || product.Ratings.ReviewCount == nil || *product.Ratings.ReviewCount != 224 {
		t.Fatalf("ratings not parsed: %#v", product.Ratings)
	}
	if product.Ratings.AverageRating != "4.68" || product.Ratings.Distribution["5"] != 196 {
		t.Fatalf("rating distribution not parsed: %#v", product.Ratings)
	}
}

func TestResolveProductRejectsPageWithoutDeal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body>no island here</body></html>`)
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	if _, err := client.ResolveProduct(context.Background(), "missing"); err == nil {
		t.Fatal("ResolveProduct accepted a page with no __NEXT_DATA__ island")
	}
}

func nextDataPage(t *testing.T) string {
	t.Helper()
	payload := map[string]any{
		"props": map[string]any{"pageProps": map[string]any{
			"deal": map[string]any{
				"id": 257731, "slug": "growify", "public_name": "Growify",
				"deal_review": map[string]any{
					"review_count": 224, "average_rating": "4.68",
					"review_count_1_tacos": 12, "review_count_2_tacos": 2,
					"review_count_3_tacos": 4, "review_count_4_tacos": 10,
					"review_count_5_tacos": 196,
				},
			},
		}},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return `<html><body><script id="__NEXT_DATA__" type="application/json">` + string(encoded) + `</script></body></html>`
}

func hasWarningContaining(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}

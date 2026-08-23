package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/cli"
)

func newReviewsFixtureServer(t *testing.T, total int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reviews are public. A cookie on any of these paths is a credential leak.
		if cookie := r.Header.Get("Cookie"); cookie != "" {
			t.Errorf("public review path %s carried cookie %q", r.URL.Path, cookie)
		}
		switch {
		case r.URL.Path == "/products/growify/reviews/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			payload, err := json.Marshal(map[string]any{"props": map[string]any{"pageProps": map[string]any{
				"deal": map[string]any{
					"id": 257731, "slug": "growify", "public_name": "Growify",
					"deal_review": map[string]any{"review_count": total, "average_rating": "4.68"},
				},
			}}})
			if err != nil {
				t.Fatalf("encode fixture: %v", err)
			}
			fmt.Fprintf(w, `<html><script id="__NEXT_DATA__" type="application/json">%s</script></html>`, payload)
		case r.URL.Path == "/api/v2/deals/257731/reviews/":
			from, _ := strconv.Atoi(r.URL.Query().Get("from"))
			size, _ := strconv.Atoi(r.URL.Query().Get("items_per_page"))
			comments := []map[string]any{}
			for i := from; i < from+size && i < total; i++ {
				comments = append(comments, map[string]any{
					"id": 5000 + i, "deal_id": 257731, "rating": 5,
					"title": fmt.Sprintf("Title %d", i), "comment": fmt.Sprintf("Body %d", i),
					"created": "2025-01-01T00:00:00Z",
					"user":    map[string]any{"id": i, "username": fmt.Sprintf("user%d", i)},
				})
			}
			writeJSON(t, w, map[string]any{
				"comments": comments,
				"meta":     map[string]any{"total": total, "count": len(comments)},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestCLIReviewsFetchesEveryPageWithoutCookie(t *testing.T) {
	server := newReviewsFixtureServer(t, 224)
	defer server.Close()

	out := runCLI(t, cli.Options{
		BaseURL: server.URL,
		// A configured session cookie must never reach the public review API.
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
	}, "reviews", "growify", "--json", "--page-size", "100")

	var report struct {
		Product struct {
			Slug   string `json:"slug"`
			DealID int64  `json:"deal_id"`
		} `json:"product"`
		Fetch struct {
			UniqueReviews int   `json:"unique_reviews"`
			ExpectedTotal *int  `json:"expected_total"`
			Complete      *bool `json:"complete"`
			Requests      int   `json:"requests"`
		} `json:"fetch"`
		Reviews []map[string]any `json:"reviews"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("reviews --json emitted invalid JSON: %v\n%s", err, out)
	}
	if report.Product.DealID != 257731 || report.Product.Slug != "growify" {
		t.Fatalf("unexpected product block: %+v", report.Product)
	}
	if report.Fetch.UniqueReviews != 224 || len(report.Reviews) != 224 {
		t.Fatalf("fetched %d reviews (%d in payload), want 224", report.Fetch.UniqueReviews, len(report.Reviews))
	}
	if report.Fetch.Complete == nil || !*report.Fetch.Complete {
		t.Fatalf("complete = %v, want true", report.Fetch.Complete)
	}
	// 100 + 100 + 24 + one empty window. The crawl proves exhaustion by reading
	// past the end rather than trusting meta.total, which would truncate
	// silently if the reported count were stale.
	if report.Fetch.Requests != 4 {
		t.Fatalf("used %d requests for 224 reviews at page size 100, want 4", report.Fetch.Requests)
	}
	if strings.Contains(out, "fixture-cookie-header") {
		t.Fatalf("output leaked the session cookie: %s", out)
	}
}

func TestCLIReviewsAnnouncesTruncationOnStderr(t *testing.T) {
	server := newReviewsFixtureServer(t, 224)
	defer server.Close()

	var out, errOut bytes.Buffer
	cmd := cli.NewRoot(cli.Options{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Out:        &out,
		Err:        &errOut,
	})
	cmd.SetArgs([]string{"reviews", "growify", "--json", "--limit", "10"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reviews --limit failed: %v", err)
	}
	if !strings.Contains(errOut.String(), "--limit 10") {
		t.Fatalf("truncation warning missing from stderr: %q", errOut.String())
	}
	// --json must stay a clean pipe even when a warning fires.
	if !json.Valid(bytes.TrimSpace(out.Bytes())) {
		t.Fatalf("stdout was not valid JSON alongside a warning: %s", out.String())
	}
}

func TestCLIReviewsRejectsUnknownProduct(t *testing.T) {
	server := newReviewsFixtureServer(t, 5)
	defer server.Close()

	cmd := cli.NewRoot(cli.Options{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Out:        &bytes.Buffer{},
		Err:        &bytes.Buffer{},
	})
	cmd.SetArgs([]string{"reviews", "does-not-exist", "--json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("reviews accepted a product slug the site does not serve")
	}
}

package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/cli"
)

func catalogRow(index int, overrides map[string]any) map[string]any {
	row := map[string]any{
		"id": 260000 + index, "slug": fmt.Sprintf("deal-%d", index),
		"public_name": fmt.Sprintf("Deal %d", index),
		"price":       49.0, "original_price": 588.0, "plus_discount_price": 44.1,
		"is_free": false, "listing_type": "marketplace",
		"get_absolute_url": fmt.Sprintf("/products/deal-%d/", index),
		"uses_codes":       false, "codes_remaining": 0,
		"uses_limited_licensing": false, "percent_claimed": -1,
		"has_ended": false, "browse_deal_status": "current",
		"dates":       map[string]any{"start_date": "2026-08-10T12:50:27Z", "end_date": nil, "timer_reason": nil},
		"deal_review": map[string]any{"review_count": 12, "average_rating": 4.68},
		"taxonomy": map[string]any{
			"category": map[string]any{"value_enumeration": "seo-analytics"},
			"group":    map[string]any{"value_enumeration": "marketing"},
		},
	}
	for key, value := range overrides {
		row[key] = value
	}
	return row
}

// catalogFixtureServer serves a catalog and refuses to carry a session cookie.
func catalogFixtureServer(t *testing.T, rowsFor func(page, perPage int) []map[string]any, total int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			t.Errorf("public catalog request carried a session cookie")
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		if perPage <= 0 {
			perPage = 100
		}
		rows := rowsFor(page, perPage)
		writeJSON(t, w, map[string]any{
			"deals": rows,
			"meta": map[string]any{
				"total_results": total, "total_pages": (total + perPage - 1) / perPage,
				"page": page, "per_page": perPage,
			},
		})
	}))
}

func pagedCatalog(total int) func(page, perPage int) []map[string]any {
	return func(page, perPage int) []map[string]any {
		rows := []map[string]any{}
		for i := (page - 1) * perPage; i < page*perPage && i < total; i++ {
			rows = append(rows, catalogRow(i, nil))
		}
		return rows
	}
}

// TestCLIDealsListNeverSendsTheSessionCookie is the credential guard for a
// public surface: `deals` must behave identically whether or not the user has a
// cookie configured.
func TestCLIDealsListNeverSendsTheSessionCookie(t *testing.T) {
	server := catalogFixtureServer(t, pagedCatalog(250), 250)
	defer server.Close()

	out := runCLI(t, cli.Options{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
		DBPath:     filepath.Join(t.TempDir(), "appsumo.db"),
	}, "deals", "list", "--json")

	var report struct {
		Fetch struct {
			UniqueDeals   int   `json:"unique_deals"`
			DeclaredTotal *int  `json:"declared_total"`
			Complete      *bool `json:"complete"`
			Requests      int   `json:"requests"`
		} `json:"fetch"`
		Deals []struct {
			Slug           string `json:"slug"`
			CodesRemaining *int   `json:"codes_remaining"`
			PercentClaimed *int   `json:"percent_claimed"`
		} `json:"deals"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("deals list --json is not valid JSON: %v\n%s", err, out)
	}
	if report.Fetch.UniqueDeals != 250 {
		t.Fatalf("collected %d deals, want 250", report.Fetch.UniqueDeals)
	}
	if report.Fetch.Complete == nil || !*report.Fetch.Complete {
		t.Fatalf("complete = %v, want true", report.Fetch.Complete)
	}
	if report.Deals[0].CodesRemaining != nil || report.Deals[0].PercentClaimed != nil {
		t.Fatalf("sentinels reached JSON output as numbers: %#v", report.Deals[0])
	}
}

// A short walk must announce itself on stderr and stay out of stdout, so a
// `--json` pipe is still clean while a human sees the crawl was incomplete.
func TestCLIDealsListAnnouncesShortWalkOnStderr(t *testing.T) {
	server := catalogFixtureServer(t, pagedCatalog(180), 363)
	defer server.Close()

	out, errOut := runCLICapturingStderr(t, cli.Options{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		DBPath:     filepath.Join(t.TempDir(), "appsumo.db"),
	}, "deals", "list", "--json")

	if !strings.Contains(errOut, "183 rows were never served") {
		t.Fatalf("short walk not announced on stderr: %q", errOut)
	}

	// stdout stays a clean pipe in the sense that matters: it parses as JSON and
	// nothing human-readable is interleaved with it. The warning is also present
	// there, as a structured field rather than as prose, because a machine
	// consumer reading only stdout must not conclude the walk was complete.
	var report struct {
		Fetch struct {
			Complete *bool `json:"complete"`
		} `json:"fetch"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stdout was not clean JSON: %v\n%s", err, out)
	}
	if report.Fetch.Complete == nil || *report.Fetch.Complete {
		t.Fatalf("a 180-of-363 walk reported complete = %v", report.Fetch.Complete)
	}
	if len(report.Warnings) == 0 {
		t.Fatal("JSON consumers got no warning about the short walk")
	}
}

// TestCLIDealsSyncRefusesToSnapshotAShortWalk is the guard that keeps the diff
// honest. Recording 180 of 363 deals would make the next diff report 183
// perfectly healthy deals as gone.
func TestCLIDealsSyncRefusesToSnapshotAShortWalk(t *testing.T) {
	server := catalogFixtureServer(t, pagedCatalog(180), 363)
	defer server.Close()

	err := runCLIExpectingError(t, cli.Options{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		DBPath:     filepath.Join(t.TempDir(), "appsumo.db"),
	}, "deals", "sync")
	if err == nil {
		t.Fatal("deals sync recorded an incomplete catalog walk")
	}
	if !strings.Contains(err.Error(), "refusing to snapshot an incomplete catalog walk") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLIDealsSyncThenDiffReportsMovement(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "appsumo.db")

	firstRun := true
	server := catalogFixtureServer(t, func(page, perPage int) []map[string]any {
		if page > 1 {
			return nil
		}
		if firstRun {
			return []map[string]any{
				catalogRow(1, map[string]any{"slug": "keeper", "price": 99.0}),
				catalogRow(2, map[string]any{"slug": "leaver", "price": 39.0}),
			}
		}
		return []map[string]any{
			catalogRow(1, map[string]any{"slug": "keeper", "price": 69.0}),
			catalogRow(3, map[string]any{"slug": "joiner", "price": 59.0}),
		}
	}, 2)
	defer server.Close()

	options := cli.Options{BaseURL: server.URL, HTTPClient: server.Client(), DBPath: dbPath}
	runCLI(t, options, "deals", "sync")
	firstRun = false
	runCLI(t, options, "deals", "sync")

	out := runCLI(t, options, "deals", "diff", "--json")
	var diff struct {
		Changes []struct {
			Kind   string `json:"kind"`
			Slug   string `json:"slug"`
			Field  string `json:"field"`
			Before string `json:"before"`
			After  string `json:"after"`
		} `json:"changes"`
	}
	if err := json.Unmarshal([]byte(out), &diff); err != nil {
		t.Fatalf("deals diff --json is not valid JSON: %v\n%s", err, out)
	}

	byKind := map[string]int{}
	var priceMove string
	for _, change := range diff.Changes {
		byKind[change.Kind]++
		if change.Slug == "keeper" && change.Field == "price" {
			priceMove = change.Before + "->" + change.After
		}
	}
	if byKind["new"] != 1 || byKind["gone"] != 1 {
		t.Fatalf("arrivals/departures wrong: %#v", diff.Changes)
	}
	if priceMove != "99.00->69.00" {
		t.Fatalf("price move wrong: %q in %#v", priceMove, diff.Changes)
	}
}

func TestCLIDealsDiffNeedsTwoSnapshots(t *testing.T) {
	err := runCLIExpectingError(t, cli.Options{
		DBPath: filepath.Join(t.TempDir(), "appsumo.db"),
	}, "deals", "diff")
	if err == nil {
		t.Fatal("deals diff succeeded with no snapshots")
	}
	// The remedy has to name a command that exists, since it is the next thing
	// the reader will type.
	if !strings.Contains(err.Error(), "appsumo deals sync") {
		t.Fatalf("error gave no usable remedy: %v", err)
	}
}

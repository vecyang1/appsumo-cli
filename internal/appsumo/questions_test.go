package appsumo_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
)

// questionFixture mirrors a captured /api/v2/deals/{id}/questions/ record. Two
// details are load-bearing and both come from live data: there is no rating
// field, and deal_id is the deal the question was *asked* on, which for a
// relaunched product is not the deal you requested.
func questionFixture(id int) map[string]any {
	return map[string]any{
		"id": id, "deal_id": 213645, "parent_id": nil, "level": 0,
		"user_id":  1766540,
		"title":    "",
		"comment":  fmt.Sprintf("Question %d: does this support agencies?", id),
		"created":  "2024-06-09T21:30:25.492994+00:00",
		"modified": "2024-06-09T21:34:10.409449+00:00",
		"pinned":   nil, "resolved": false, "approved": true, "edited": false,
		"up_votes": 0, "down_votes": 0, "answer_type": nil, "followup": false,
		"purchased": false, "status": "Approved",
		"display_path": fmt.Sprintf("/products/growify/questions/question-%d/", id),
		"user": map[string]any{
			"id": 1766540, "username": "DaveC",
			"date_joined": "2017-06-29T05:17:34Z", "deals_purchased": 252, "has_plus": false,
		},
		"children": []map[string]any{{
			"id": 900000 + id, "deal_id": 213645, "parent_id": id, "level": 1,
			"comment": "Yes — unlimited domains and users.", "resolved": true,
			"created": "2024-06-09T21:36:11.708410+00:00",
			"user":    map[string]any{"id": 3105426, "username": "founder"},
		}},
	}
}

func questionsServer(t *testing.T, total int, seenPaths *[]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			t.Errorf("question request carried a session cookie: %s", r.URL.Path)
		}
		if seenPaths != nil {
			*seenPaths = append(*seenPaths, r.URL.Path)
		}
		from, _ := strconv.Atoi(r.URL.Query().Get("from"))
		size, _ := strconv.Atoi(r.URL.Query().Get("items_per_page"))
		if size <= 0 {
			size = 20
		}
		comments := []map[string]any{}
		for i := from; i < from+size && i < total; i++ {
			comments = append(comments, questionFixture(1117000+i))
		}
		writeJSON(t, w, map[string]any{
			"comments": comments,
			"meta":     map[string]any{"total": total, "count": len(comments)},
		})
	}))
}

func TestFetchAllQuestionsWalksEveryOffset(t *testing.T) {
	var paths []string
	server := questionsServer(t, 350, &paths)
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
	})

	result, err := client.FetchAllQuestions(context.Background(), appsumo.ThreadQuery{DealID: 257731, PageSize: 100}, 0)
	if err != nil {
		t.Fatalf("FetchAllQuestions returned error: %v", err)
	}
	if len(result.Questions) != 350 {
		t.Fatalf("collected %d questions, want 350", len(result.Questions))
	}
	if result.ExpectedTotal == nil || *result.ExpectedTotal != 350 {
		t.Fatalf("expected total = %v, want 350", result.ExpectedTotal)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("healthy crawl emitted warnings: %v", result.Warnings)
	}
	for _, path := range paths {
		if path != "/api/v2/deals/257731/questions/" {
			t.Fatalf("unexpected path %s", path)
		}
	}

	first := result.Questions[0]
	if !first.Answered() || first.Children[0].User.Username != "founder" {
		t.Fatalf("nested answer was dropped: %#v", first.Children)
	}
	// Q&A carries across a product's earlier deal runs; asserting equality here
	// would fail on every relaunched product.
	if int64(first.DealID) != 213645 {
		t.Fatalf("deal_id was rewritten to the requested deal: %d", first.DealID)
	}
}

// TestFetchAllQuestionsInheritsTheOffsetGuard is the evidence for the claim that
// sharing the crawl protects both surfaces: the duplicate-window guard was
// written for reviews, and this asserts it fires on the Q&A path too.
func TestFetchAllQuestionsInheritsTheOffsetGuard(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		comments := []map[string]any{}
		for i := 0; i < 5; i++ {
			comments = append(comments, questionFixture(2000+i))
		}
		writeJSON(t, w, map[string]any{
			"comments": comments,
			"meta":     map[string]any{"total": 350, "count": len(comments)},
		})
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllQuestions(context.Background(), appsumo.ThreadQuery{DealID: 1, PageSize: 5}, 0)
	if err != nil {
		t.Fatalf("FetchAllQuestions returned error: %v", err)
	}
	if requests > 2 {
		t.Fatalf("kept requesting an endpoint that ignores `from`: %d requests", requests)
	}
	if !hasWarningContaining(result.Warnings, "ignore `from`") {
		t.Fatalf("no warning about the ignored offset: %v", result.Warnings)
	}
	// The warning must name questions, not reviews: it is raised from code the
	// two surfaces share, and a message that names the wrong surface sends the
	// reader looking at the wrong command.
	if !hasWarningContaining(result.Warnings, "questions") {
		t.Fatalf("shared warning did not name this surface: %v", result.Warnings)
	}
	if hasWarningContaining(result.Warnings, "reviews") {
		t.Fatalf("shared warning named the other surface: %v", result.Warnings)
	}
}

func TestFetchAllQuestionsHonoursLimit(t *testing.T) {
	server := questionsServer(t, 350, nil)
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllQuestions(context.Background(), appsumo.ThreadQuery{DealID: 1, PageSize: 100}, 25)
	if err != nil {
		t.Fatalf("FetchAllQuestions returned error: %v", err)
	}
	if len(result.Questions) != 25 {
		t.Fatalf("collected %d questions, want 25", len(result.Questions))
	}
	if !result.Truncated || !hasWarningContaining(result.Warnings, "more questions exist") {
		t.Fatalf("truncation was not announced: truncated=%v warnings=%v", result.Truncated, result.Warnings)
	}
}

// The shared crawl must not certify a thread walk that stopped on the ignored-
// offset anomaly, even when the collected count happens to match meta.total.
func TestFetchAllQuestionsWillNotCertifyAnAnomalousCrawl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		from, _ := strconv.Atoi(r.URL.Query().Get("from"))
		if from >= 5 {
			from = 0 // ignore the offset, exactly as the live decoy does
		}
		comments := []map[string]any{}
		for i := from; i < 5; i++ {
			comments = append(comments, questionFixture(3000+i))
		}
		writeJSON(t, w, map[string]any{
			"comments": comments,
			"meta":     map[string]any{"total": 5, "count": len(comments)},
		})
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	result, err := client.FetchAllQuestions(context.Background(), appsumo.ThreadQuery{DealID: 1, PageSize: 5}, 0)
	if err != nil {
		t.Fatalf("FetchAllQuestions returned error: %v", err)
	}
	if len(result.Questions) != 5 || result.ExpectedTotal == nil || *result.ExpectedTotal != 5 {
		t.Fatalf("precondition wrong: %d of %v", len(result.Questions), result.ExpectedTotal)
	}
	if !result.Truncated {
		t.Fatal("a crawl that stopped on the ignored-offset anomaly was not marked truncated")
	}
	if !hasWarningContaining(result.Warnings, "ignore `from`") {
		t.Fatalf("the anomaly was not announced: %v", result.Warnings)
	}
}

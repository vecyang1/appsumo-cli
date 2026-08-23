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

func questionRow(id int, replies int) map[string]any {
	children := []map[string]any{}
	for i := 0; i < replies; i++ {
		children = append(children, map[string]any{
			"id": 900000 + id*10 + i, "deal_id": 213645, "parent_id": id, "level": 1,
			"comment": "Answer", "resolved": true,
			"user": map[string]any{"id": 3105426, "username": "founder"},
		})
	}
	return map[string]any{
		"id": id, "deal_id": 213645, "parent_id": nil, "level": 0,
		"title": "", "comment": fmt.Sprintf("Question %d", id),
		"created": "2024-06-09T21:30:25.492994+00:00", "status": "Approved",
		"up_votes": 0, "down_votes": 0, "resolved": false, "approved": true,
		"display_path": fmt.Sprintf("/products/growify/questions/q-%d/", id),
		"user":         map[string]any{"id": 1766540, "username": "asker"},
		"children":     children,
	}
}

// questionsFixtureServer serves the product page and the Q&A API, and fails the
// test if either request carries a session cookie.
func questionsFixtureServer(t *testing.T, total int, unanswered int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" {
			t.Errorf("public Q&A request carried a session cookie: %s", r.URL.Path)
		}
		if strings.HasSuffix(r.URL.Path, "/reviews/") && !strings.Contains(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<html><body><script id="__NEXT_DATA__" type="application/json">%s</script></body></html>`,
				`{"props":{"pageProps":{"deal":{"id":257731,"slug":"growify","public_name":"Growify"}}}}`)
			return
		}
		from, _ := strconv.Atoi(r.URL.Query().Get("from"))
		size, _ := strconv.Atoi(r.URL.Query().Get("items_per_page"))
		if size <= 0 {
			size = 100
		}
		comments := []map[string]any{}
		for i := from; i < from+size && i < total; i++ {
			replies := 2
			if i < unanswered {
				replies = 0
			}
			comments = append(comments, questionRow(1117000+i, replies))
		}
		writeJSON(t, w, map[string]any{
			"comments": comments,
			"meta":     map[string]any{"total": total, "count": len(comments)},
		})
	}))
}

func TestCLIQuestionsFetchesEveryPageWithoutCookie(t *testing.T) {
	server := questionsFixtureServer(t, 350, 3)
	defer server.Close()

	out := runCLI(t, cli.Options{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
		DBPath:     filepath.Join(t.TempDir(), "appsumo.db"),
	}, "questions", "growify", "--json")

	var report struct {
		Product struct {
			DealID int64 `json:"deal_id"`
		} `json:"product"`
		Fetch struct {
			UniqueQuestions int   `json:"unique_questions"`
			Answered        int   `json:"answered"`
			Complete        *bool `json:"complete"`
			Requests        int   `json:"requests"`
			Saved           *int  `json:"saved_rows"`
		} `json:"fetch"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("questions --json is not valid JSON: %v\n%s", err, out)
	}
	if report.Fetch.UniqueQuestions != 350 {
		t.Fatalf("collected %d questions, want 350", report.Fetch.UniqueQuestions)
	}
	// 350 threads at 100 per window is 4 full windows plus one empty read that
	// proves exhaustion rather than trusting meta.total.
	if report.Fetch.Requests != 5 {
		t.Fatalf("made %d requests, want 5", report.Fetch.Requests)
	}
	if report.Fetch.Complete == nil || !*report.Fetch.Complete {
		t.Fatalf("complete = %v, want true", report.Fetch.Complete)
	}
	// The answered count must discriminate. A rollup that reports every thread
	// answered is indistinguishable from one that never checked.
	if report.Fetch.Answered != 347 {
		t.Fatalf("answered = %d, want 347 of 350", report.Fetch.Answered)
	}
	if report.Fetch.Saved != nil {
		t.Fatalf("questions wrote to the database without --save: %v", report.Fetch.Saved)
	}
}

func TestCLIQuestionsSaveWritesToTheDatabase(t *testing.T) {
	server := questionsFixtureServer(t, 12, 2)
	defer server.Close()

	dbPath := filepath.Join(t.TempDir(), "appsumo.db")
	options := cli.Options{BaseURL: server.URL, HTTPClient: server.Client(), DBPath: dbPath}
	runCLI(t, options, "questions", "growify", "--save")

	out := runCLI(t, options, "sql",
		"select count(*) as n from product_comments where kind = 'question'", "--json")
	if !strings.Contains(out, `"n": "32"`) {
		t.Fatalf("stored question rows not found (12 threads + 20 replies): %s", out)
	}

	answered := runCLI(t, options, "sql",
		"select count(*) as n from product_comments where kind = 'question' and answered = 1", "--json")
	if !strings.Contains(answered, `"n": "10"`) {
		t.Fatalf("answered flag not stored discriminatingly: %s", answered)
	}
}

func TestCLIQuestionsRejectsUnknownProduct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body>no island</body></html>`)
	}))
	defer server.Close()

	err := runCLIExpectingError(t, cli.Options{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		DBPath:     filepath.Join(t.TempDir(), "appsumo.db"),
	}, "questions", "not-a-product")
	if err == nil {
		t.Fatal("questions accepted a page with no product data")
	}
	if !strings.Contains(err.Error(), "__NEXT_DATA__") {
		t.Fatalf("error did not say what was missing: %v", err)
	}
}

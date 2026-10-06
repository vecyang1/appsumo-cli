package appsumo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanProductSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"poppy-ai", "poppy-ai"},
		{"/poppy-ai/", "poppy-ai"},
		{"https://appsumo.com/products/poppy-ai/", "poppy-ai"},
		{"https://appsumo.com/products/poppy-ai", "poppy-ai"},
		{"https://appsumo.com/products/poppy-ai/reviews/", "poppy-ai"},
		{"http://appsumo.com/products/test-tool/?ref=123", "test-tool"},
	}

	for _, tc := range tests {
		got := CleanProductSlug(tc.input)
		if got != tc.expected {
			t.Errorf("CleanProductSlug(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestFetchProductDeal(t *testing.T) {
	sampleHTML := `<!DOCTYPE html><html><head>
<script id="__NEXT_DATA__" type="application/json">
{
  "props": {
    "pageProps": {
      "deal": {
        "id": 255707,
        "slug": "poppy-ai",
        "public_name": "Poppy AI",
        "product_url": "https://getpoppy.ai/",
        "deal_status": "Ended",
        "refundable_days": 60,
        "plans": [
          {
            "id": 287128,
            "tier": 1,
            "plan_type": "Lifetime Deal",
            "price": "279.00",
            "original_price": "649.00",
            "is_active": true,
            "plan_features": [{"feature": "<b>500</b> Monthly credits"}]
          }
        ],
        "default_plan_terms": [{"id": 1, "text": "Lifetime access"}],
        "common_features": [{"id": 2, "text": "MCPs"}],
        "faqs": [{"id": 3, "question": "What is Poppy?", "answer": "An AI workspace"}],
        "founders": {
          "data": {
            "company_size": "11-50",
            "headquarters": "San Francisco, US",
            "founded_at": "2024-05-11",
            "product_stage": {"label": "Expansion", "value": "expansion", "description": ""},
            "financial_stage": {"label": "Bootstrapped", "value": "bootstrapped", "description": ""},
            "is_verified": true
          },
          "links": [{"id": 4, "type": "twitter", "title": "Twitter", "url": "https://x.com/poppy"}],
          "posts": [{"id": 5, "title": "Hello", "message": "<p>Welcome!</p>", "created": "2024-05-12", "user": {"username": "qazi", "first_name": "Rafeh", "last_name": "Qazi", "avatar": "", "linkedin_url": ""}}]
        },
        "deal_review": {
          "review_count": 161,
          "average_rating": "4.83",
          "review_count_5_tacos": 153,
          "review_count_1_tacos": 5
        }
      }
    }
  }
}
</script>
</head><body></body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(sampleHTML))
	}))
	defer server.Close()

	client := NewClient(ClientOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	deal, err := client.FetchProductDeal(context.Background(), "poppy-ai")
	if err != nil {
		t.Fatalf("FetchProductDeal failed: %v", err)
	}

	if deal.PublicName != "Poppy AI" {
		t.Errorf("PublicName = %q, want 'Poppy AI'", deal.PublicName)
	}
	if deal.Slug != "poppy-ai" {
		t.Errorf("Slug = %q, want 'poppy-ai'", deal.Slug)
	}
	if len(deal.Plans) != 1 {
		t.Errorf("Plans count = %d, want 1", len(deal.Plans))
	}
	if len(deal.FAQs) != 1 {
		t.Errorf("FAQs count = %d, want 1", len(deal.FAQs))
	}
	if len(deal.Founders.Posts) != 1 {
		t.Errorf("Founders posts count = %d, want 1", len(deal.Founders.Posts))
	}
	if deal.Ratings == nil || *deal.Ratings.ReviewCount != 161 {
		t.Errorf("Ratings count = %v, want 161", deal.Ratings)
	}

	// Test SaveToDirectory & GenerateMarkdownDossier
	archive := &FullProductArchive{
		ScrapedAt: "2026-10-07T02:00:00Z",
		Slug:      deal.Slug,
		Deal:      *deal,
		Reviews: []Review{
			{
				ID:      101,
				Title:   "Great tool",
				Comment: "Really helps with creative workflows",
				Rating:  intPtr(5),
				User:    ReviewUser{Username: "happyuser"},
			},
		},
		Questions: []Question{
			{
				ID:      201,
				Title:   "Does it support BYOK?",
				Comment: "Can I bring my own API key?",
				User:    QuestionUser{Username: "asker"},
			},
		},
	}

	tempDir, err := os.MkdirTemp("", "appsumo_scrape_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	if err := archive.SaveToDirectory(tempDir); err != nil {
		t.Fatalf("SaveToDirectory failed: %v", err)
	}

	for _, name := range []string{"deal.json", "reviews.json", "questions.json", "faqs.json", "founders.json", "full_archive.json", "DOSSIER.md"} {
		path := filepath.Join(tempDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s to exist, got err: %v", name, err)
		}
	}
}

func intPtr(i int) *int {
	return &i
}

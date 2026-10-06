package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScrapeCmd(t *testing.T) {
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
          "links": [],
          "posts": []
        },
        "deal_review": {
          "review_count": 0,
          "average_rating": "5.00"
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

	tempDir, err := os.MkdirTemp("", "cli_scrape_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}

	root := NewRoot(Options{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		Out:        outBuf,
		Err:        errBuf,
	})

	outTarget := filepath.Join(tempDir, "scraped_data")
	root.SetArgs([]string{"scrape", "poppy-ai", "--out-dir", outTarget, "--no-reviews", "--no-questions"})

	if err := root.Execute(); err != nil {
		t.Fatalf("root.Execute() failed: %v, stderr: %s", err, errBuf.String())
	}

	stdout := outBuf.String()
	if !strings.Contains(stdout, "APPSUMO PRODUCT SCRAPE COMPLETE") {
		t.Errorf("expected complete message in output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Poppy AI") {
		t.Errorf("expected product name in output, got: %s", stdout)
	}

	for _, name := range []string{"deal.json", "faqs.json", "full_archive.json", "DOSSIER.md"} {
		p := filepath.Join(outTarget, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to exist, got err: %v", name, err)
		}
	}
}

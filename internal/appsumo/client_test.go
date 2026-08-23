package appsumo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
)

func TestFetchAllProductsPaginatesAndSendsCookie(t *testing.T) {
	var seenCookies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCookies = append(seenCookies, r.Header.Get("Cookie"))
		if r.URL.Path != "/api/v2/account/products/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			writeJSON(t, w, map[string]any{
				"products": map[string]any{
					"count":    2,
					"next":     serverURLPlaceholder + "/api/v2/account/products/?page=2",
					"previous": nil,
					"results": []map[string]any{{
						"id": 1, "uuid": "u1", "invoice_uuid": "i1", "name": "Letterly", "slug": "letterly", "status": "activated", "plan_name": "Plan A",
					}},
				},
			})
			return
		}
		writeJSON(t, w, map[string]any{
			"products": map[string]any{
				"count":    2,
				"next":     nil,
				"previous": serverURLPlaceholder + "/api/v2/account/products/?page=1",
				"results": []map[string]any{{
					"id": "2", "uuid": "u2", "invoice_uuid": "i2", "name": "Buildfast", "slug": "buildfast", "status": "expired", "plan_name": "Plan B",
				}},
			},
		})
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
	})

	products, total, err := client.FetchAllProducts(context.Background())
	if err != nil {
		t.Fatalf("FetchAllProducts returned error: %v", err)
	}
	if total != 2 || len(products) != 2 {
		t.Fatalf("expected total=2 and 2 products, got total=%d len=%d", total, len(products))
	}
	if products[0].Name != "Letterly" || products[1].Name != "Buildfast" {
		t.Fatalf("unexpected products: %#v", products)
	}
	for _, cookie := range seenCookies {
		if cookie != "fixture-cookie-header" {
			t.Fatalf("request cookie = %q, want session cookie", cookie)
		}
	}
}

func TestCookieSentToAppSumoHost(t *testing.T) {
	var seenCookie string
	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL: "https://appsumo.com",
		Cookie:  "fixture-cookie-header",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			seenCookie = req.Header.Get("Cookie")
			return jsonResponse(200, `{"user":{"is_authenticated":true}}`), nil
		})},
	})

	if _, err := client.AuthStatus(context.Background()); err != nil {
		t.Fatalf("AuthStatus returned error: %v", err)
	}
	if seenCookie != "fixture-cookie-header" {
		t.Fatalf("cookie header = %q, want AppSumo cookie", seenCookie)
	}
}

func TestCookieNotSentToArbitraryBaseURL(t *testing.T) {
	var seenCookie string
	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL: "https://not-appsumo.example",
		Cookie:  "fixture-cookie-header",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			seenCookie = req.Header.Get("Cookie")
			return jsonResponse(200, `{"user":{"is_authenticated":true}}`), nil
		})},
	})

	if _, err := client.AuthStatus(context.Background()); err != nil {
		t.Fatalf("AuthStatus returned error: %v", err)
	}
	if seenCookie != "" {
		t.Fatalf("cookie leaked to arbitrary host: %q", seenCookie)
	}
}

func TestFetchProductsCSV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/account/products/download/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Cookie") != "fixture-cookie-header" {
			t.Fatalf("missing cookie header")
		}
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("Product name,License Key / Code\nTool,secret-code\n"))
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
	})

	got, err := client.FetchProductsCSV(context.Background())
	if err != nil {
		t.Fatalf("FetchProductsCSV returned error: %v", err)
	}
	if string(got) != "Product name,License Key / Code\nTool,secret-code\n" {
		t.Fatalf("unexpected CSV: %q", string(got))
	}
}

func TestFetchProductsCSVFailsWhenResponseExceedsLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/account/products/download/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte("Product name,License Key / Code\n"))
		_, _ = w.Write([]byte(strings.Repeat("x", (10<<20)+1)))
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
	})

	if _, err := client.FetchProductsCSV(context.Background()); err == nil || !strings.Contains(err.Error(), "response body exceeded") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestFetchProductsPageFailsWhenJSONResponseExceedsLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/account/products/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Repeat(" ", (10<<20)+1)))
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
	})

	if _, err := client.FetchProductsPage(context.Background(), 1); err == nil || !strings.Contains(err.Error(), "response body exceeded") {
		t.Fatalf("expected oversized response error, got %v", err)
	}
}

func TestAuthStatusDoesNotExposeSensitiveUserFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sessions/current/" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeJSON(t, w, map[string]any{
			"user": map[string]any{
				"is_authenticated": true,
				"email":            "fixture-account-email",
				"customer_id":      "fixture-customer-id",
			},
		})
	}))
	defer server.Close()

	client := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
	})

	status, err := client.AuthStatus(context.Background())
	if err != nil {
		t.Fatalf("AuthStatus returned error: %v", err)
	}
	if !status.Authenticated {
		t.Fatalf("expected authenticated status")
	}
	if status.Email != "" || status.CustomerID != "" {
		t.Fatalf("auth status exposed sensitive fields: %#v", status)
	}
}

const serverURLPlaceholder = "https://appsumo.test"

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// TestErrorsNameOnlyWhatTheirCallerShares is the guard for a message raised from
// shared code. Every command routes through getJSON, so an auth diagnosis
// written there is printed verbatim by `deals`, `reviews`, and `questions`,
// which are never authenticated. A reader who ran a public command and is told
// their session may be unauthenticated stops before the line that would have
// helped — a right answer under a wrong question does not get read.
func TestErrorsNameOnlyWhatTheirCallerShares(t *testing.T) {
	// Answers every request with an HTML sign-in page, as AppSumo does.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body>Sign in</body></html>`)
	}))
	defer server.Close()

	noCookie := appsumo.NewClient(appsumo.ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})

	// A public surface must not be told anything about credentials.
	_, _, err := noCookie.FetchDealsPage(context.Background(), 1, 10, "newest")
	if err == nil {
		t.Fatal("a public catalog request accepted an HTML response")
	}
	for _, forbidden := range []string{"cookie", "Cookie", "unauthenticated", "session", "APPSUMO_COOKIE"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Errorf("public catalog error mentions %q, which is not this caller's problem: %v", forbidden, err)
		}
	}
	if !strings.Contains(err.Error(), "instead of json") {
		t.Errorf("public catalog error did not say what came back: %v", err)
	}

	// The account surface must carry a remedy, and it must name a real source.
	_, err = noCookie.AuthStatus(context.Background())
	if err == nil {
		t.Fatal("an account request accepted an HTML response")
	}
	if !strings.Contains(err.Error(), "APPSUMO_COOKIE") || !strings.Contains(err.Error(), "--cookie-file") {
		t.Errorf("account error gave no usable remedy: %v", err)
	}

	// With a cookie configured, "none is configured" would be false. The two
	// states need different remedies because the user's next action differs.
	withCookie := appsumo.NewClient(appsumo.ClientOptions{
		BaseURL: server.URL, Cookie: "fixture-cookie-header", HTTPClient: server.Client(),
	})
	_, err = withCookie.AuthStatus(context.Background())
	if err == nil {
		t.Fatal("an account request accepted an HTML response")
	}
	if strings.Contains(err.Error(), "no AppSumo session cookie is configured") {
		t.Errorf("told a user with a configured cookie that they have none: %v", err)
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("a rejected cookie was not diagnosed as expired: %v", err)
	}
}

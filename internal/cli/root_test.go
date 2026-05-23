package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
	"github.com/vecyang1/appsumo-cli/internal/cli"
	"github.com/vecyang1/appsumo-cli/internal/store"
)

func TestCLIAuthProductsExportSyncSearchAndSQL(t *testing.T) {
	server := newFixtureServer(t)
	defer server.Close()
	dbPath := filepath.Join(t.TempDir(), "appsumo.db")

	authOut := runCLI(t, cli.Options{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
		DBPath:     dbPath,
	}, "auth", "status", "--json")
	if !strings.Contains(authOut, `"authenticated": true`) {
		t.Fatalf("auth status did not report authenticated: %s", authOut)
	}
	if strings.Contains(authOut, "fixture-account-email") || strings.Contains(authOut, "fixture-customer-id") {
		t.Fatalf("auth status leaked sensitive user fields: %s", authOut)
	}

	listOut := runCLI(t, cli.Options{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
		DBPath:     dbPath,
	}, "products", "list", "--json")
	if !strings.Contains(listOut, "Letterly") || !strings.Contains(listOut, "Buildfast") {
		t.Fatalf("products list missing expected product names: %s", listOut)
	}

	searchOut := runCLI(t, cli.Options{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
		DBPath:     dbPath,
	}, "products", "search", "letter", "--json")
	if !strings.Contains(searchOut, "Letterly") || strings.Contains(searchOut, "Buildfast") {
		t.Fatalf("products search returned wrong results: %s", searchOut)
	}

	csvOut := runCLI(t, cli.Options{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
		DBPath:     dbPath,
	}, "products", "export", "--format", "csv")
	if strings.Contains(csvOut, "SHOULD_REDACT") {
		t.Fatalf("products export leaked license code: %s", csvOut)
	}
	if !strings.Contains(csvOut, "[REDACTED]") {
		t.Fatalf("products export did not redact license code: %s", csvOut)
	}

	syncOut := runCLI(t, cli.Options{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
		DBPath:     dbPath,
	}, "sync")
	if !strings.Contains(syncOut, "synced 2 products") {
		t.Fatalf("sync output missing summary: %s", syncOut)
	}

	localSearchOut := runCLI(t, cli.Options{DBPath: dbPath}, "search", "build", "--json")
	if !strings.Contains(localSearchOut, "Buildfast") || strings.Contains(localSearchOut, "Letterly") {
		t.Fatalf("local search returned wrong results: %s", localSearchOut)
	}

	sqlOut := runCLI(t, cli.Options{DBPath: dbPath}, "sql", "select name, status from products where status = 'expired'", "--json")
	if !strings.Contains(sqlOut, "Buildfast") || !strings.Contains(sqlOut, "expired") {
		t.Fatalf("sql returned wrong rows: %s", sqlOut)
	}
}

func TestCLISQLRejectsWriteStatements(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "appsumo.db")
	var out, errOut bytes.Buffer
	cmd := cli.NewRoot(cli.Options{Out: &out, Err: &errOut, DBPath: dbPath})
	cmd.SetArgs([]string{"sql", "delete from products"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected delete query to fail")
	}
}

func TestCLIRejectsCSVRedactionOptOut(t *testing.T) {
	server := newFixtureServer(t)
	defer server.Close()
	dbPath := filepath.Join(t.TempDir(), "appsumo.db")
	var out, errOut bytes.Buffer
	cmd := cli.NewRoot(cli.Options{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
		DBPath:     dbPath,
		Out:        &out,
		Err:        &errOut,
	})
	cmd.SetArgs([]string{"products", "export", "--format", "csv", "--redact-codes=false"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected --redact-codes opt-out to be rejected")
	}
}

func TestCLISQLReturnsTextOutputWriteError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "appsumo.db")
	ctx := context.Background()
	db, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := db.UpsertProducts(ctx, []appsumo.Product{
		{ID: 1, UUID: "u1", InvoiceUUID: "i1", Name: "Letterly", Slug: "letterly", Status: "activated", PlanName: "Tier 1"},
	}); err != nil {
		t.Fatalf("UpsertProducts returned error: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	cmd := cli.NewRoot(cli.Options{Out: failingWriter{}, DBPath: dbPath})
	cmd.SetArgs([]string{"sql", "select name from products"})
	if err := cmd.Execute(); !errors.Is(err, errWriterClosed) {
		t.Fatalf("expected writer error, got %v", err)
	}
}

func TestCLIReturnsDefaultDBDirectoryCreationError(t *testing.T) {
	tempDir := t.TempDir()
	homeAsFile := filepath.Join(tempDir, "home-file")
	if err := os.WriteFile(homeAsFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write home file: %v", err)
	}
	t.Setenv("HOME", homeAsFile)
	t.Setenv("APPSUMO_DB_PATH", "")

	var out, errOut bytes.Buffer
	cmd := cli.NewRoot(cli.Options{Out: &out, Err: &errOut})
	cmd.SetArgs([]string{"search", "letter"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "create default database directory") {
		t.Fatalf("expected default DB directory error, got %v", err)
	}
}

var errWriterClosed = errors.New("writer closed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriterClosed
}

func runCLI(t *testing.T, options cli.Options, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	options.Out = &out
	options.Err = &errOut
	cmd := cli.NewRoot(options)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("appsumo %s failed: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), err, out.String(), errOut.String())
	}
	return out.String()
}

func newFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "fixture-cookie-header" {
			http.Error(w, "missing cookie", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/sessions/current/":
			writeJSON(t, w, map[string]any{
				"user": map[string]any{
					"is_authenticated": true,
					"email":            "fixture-account-email",
					"customer_id":      "fixture-customer-id",
				},
			})
		case "/api/v2/account/products/":
			page := r.URL.Query().Get("page")
			if page == "" || page == "1" {
				writeJSON(t, w, productsEnvelope(2, server.URL+"/api/v2/account/products/?page=2", nil, []map[string]any{{
					"id": 1, "uuid": "u1", "invoice_uuid": "i1", "name": "Letterly", "slug": "letterly", "status": "activated", "plan_name": "Tier 1", "support_email": "fixture-support-contact",
				}}))
				return
			}
			writeJSON(t, w, productsEnvelope(2, nil, server.URL+"/api/v2/account/products/?page=1", []map[string]any{{
				"id": 2, "uuid": "u2", "invoice_uuid": "i2", "name": "Buildfast", "slug": "buildfast", "status": "expired", "plan_name": "Tier 2",
			}}))
		case "/api/v2/account/products/download/":
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("Product name,License Key / Code,Status\nTool,SHOULD_REDACT,Activated\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func productsEnvelope(count int, next any, previous any, results []map[string]any) map[string]any {
	return map[string]any{
		"products": map[string]any{
			"count":    count,
			"next":     next,
			"previous": previous,
			"results":  results,
		},
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

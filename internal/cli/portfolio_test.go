package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/cli"
)

// TestCLIPortfolioDoesNotBelieveTheRedemptionFlag is the end-to-end version of
// the account defect: the fixture reports is_redeemed false on every product,
// exactly as the live API does, including on products the buyer has redeemed.
// A rollup that trusted the flag would print an action list of every product.
func TestCLIPortfolioDoesNotBelieveTheRedemptionFlag(t *testing.T) {
	server := newFixtureServer(t)
	defer server.Close()
	dbPath := filepath.Join(t.TempDir(), "appsumo.db")

	options := cli.Options{
		BaseURL:    server.URL,
		Cookie:     "fixture-cookie-header",
		HTTPClient: server.Client(),
		DBPath:     dbPath,
	}
	runCLI(t, options, "sync")

	out, errOut := runCLICapturingStderr(t, options, "portfolio", "--json")

	var report struct {
		Source  string `json:"source"`
		Summary struct {
			Products           int            `json:"products"`
			ByRedemption       map[string]int `json:"by_redemption"`
			AwaitingRedemption []struct {
				Slug string `json:"slug"`
			} `json:"awaiting_redemption"`
			Warnings []string `json:"warnings"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("portfolio --json is not valid JSON: %v\n%s", err, out)
	}
	if report.Source != "local" {
		t.Fatalf("portfolio read %q, want the local database by default", report.Source)
	}
	if report.Summary.Products == 0 {
		t.Fatal("portfolio summarised no products after a sync")
	}
	// The fixture products carry a redeem_date, so none of them are awaiting
	// redemption even though every is_redeemed is false.
	if len(report.Summary.AwaitingRedemption) != 0 {
		t.Fatalf("the flag was believed; action list should be empty: %#v", report.Summary.AwaitingRedemption)
	}
	if report.Summary.ByRedemption["redeemed"] != report.Summary.Products {
		t.Fatalf("redemption derived wrong: %#v", report.Summary.ByRedemption)
	}
	if len(report.Summary.Warnings) == 0 {
		t.Fatal("JSON consumers got no warning that the flag disagrees with the dates")
	}
	if !strings.Contains(errOut, "is_redeemed is false on") {
		t.Fatalf("the flag disagreement was not announced on stderr: %q", errOut)
	}
}

// An empty database must say what to do, not print a confident set of zeroes.
func TestCLIPortfolioOnAnEmptyDatabaseGivesARemedy(t *testing.T) {
	options := cli.Options{DBPath: filepath.Join(t.TempDir(), "appsumo.db")}
	out, errOut := runCLICapturingStderr(t, options, "portfolio")

	if !strings.Contains(errOut, "appsumo sync") {
		t.Fatalf("no remedy for an empty database: %q", errOut)
	}
	if !strings.Contains(out, "0 products") {
		t.Fatalf("unexpected output for an empty database: %q", out)
	}
}

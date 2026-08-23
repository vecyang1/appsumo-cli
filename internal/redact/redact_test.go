package redact_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/redact"
)

func TestCSVRedactsLicenseCodeColumnValues(t *testing.T) {
	input := "Product name,Plan name,License Key / Code,Status\nTool,Plan,SHOULD_REDACT,Activated\n"

	got, err := redact.CSV([]byte(input))
	if err != nil {
		t.Fatalf("CSV returned error: %v", err)
	}
	out := string(got)
	if strings.Contains(out, "SHOULD_REDACT") {
		t.Fatalf("CSV output leaked license code: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("CSV output did not include redaction marker: %s", out)
	}
	if !strings.Contains(out, "Tool") || !strings.Contains(out, "Activated") {
		t.Fatalf("CSV redaction removed non-sensitive product fields: %s", out)
	}
}

func TestJSONValueRedactsNestedSensitiveKeys(t *testing.T) {
	var input map[string]any
	if err := json.Unmarshal([]byte(`{
		"name": "Tool",
		"license_key": "SHOULD_REDACT_ONE",
		"invoice_item_license_key": "SHOULD_REDACT_TWO",
		"nested": {
			"License Key / Code": "SHOULD_REDACT_THREE",
			"status": "activated"
		}
	}`), &input); err != nil {
		t.Fatal(err)
	}

	got := redact.JSONValue(input)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal redacted JSON: %v", err)
	}
	out := string(encoded)
	for _, leaked := range []string{"SHOULD_REDACT_ONE", "SHOULD_REDACT_TWO", "SHOULD_REDACT_THREE"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("JSON output leaked %q: %s", leaked, out)
		}
	}
	if strings.Count(out, "[REDACTED]") != 3 {
		t.Fatalf("expected three redaction markers, got: %s", out)
	}
	if !strings.Contains(out, "Tool") || !strings.Contains(out, "activated") {
		t.Fatalf("JSON redaction removed non-sensitive fields: %s", out)
	}
}

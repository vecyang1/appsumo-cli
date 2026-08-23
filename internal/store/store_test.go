package store_test

import (
	"context"
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
	"github.com/vecyang1/appsumo-cli/internal/store"
)

func TestStoreUpsertSearchAndReadOnlySQL(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	products := []appsumo.Product{
		{ID: 1, UUID: "u1", InvoiceUUID: "i1", Name: "Letterly", Slug: "letterly", Status: "activated", PlanName: "Tier 1", SupportEmail: "fixture-support-contact"},
		{ID: 2, UUID: "u2", InvoiceUUID: "i2", Name: "Buildfast", Slug: "buildfast", Status: "expired", PlanName: "Tier 2"},
	}
	if err := db.UpsertProducts(ctx, products); err != nil {
		t.Fatalf("UpsertProducts returned error: %v", err)
	}

	found, err := db.SearchProducts(ctx, "letter")
	if err != nil {
		t.Fatalf("SearchProducts returned error: %v", err)
	}
	if len(found) != 1 || found[0].Name != "Letterly" {
		t.Fatalf("unexpected search results: %#v", found)
	}

	rows, err := db.QueryReadOnly(ctx, "select name, status from products where status = 'expired'")
	if err != nil {
		t.Fatalf("QueryReadOnly returned error: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "Buildfast" || rows[0]["status"] != "expired" {
		t.Fatalf("unexpected SQL rows: %#v", rows)
	}

	if err := db.UpsertProducts(ctx, []appsumo.Product{
		{ID: 3, UUID: "u3", InvoiceUUID: "i3", Name: "Fresh", Slug: "fresh", Status: "active", PlanName: "Tier 3"},
	}); err != nil {
		t.Fatalf("UpsertProducts after read-only query returned error: %v", err)
	}
}

func TestQueryReadOnlyRejectsWrites(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	if _, err := db.QueryReadOnly(ctx, "delete from products"); err == nil {
		t.Fatalf("expected write query rejection")
	}
}

// A multi-line SELECT is the normal shape for anything worth querying. The guard
// used to test for the literal "select " and rejected a query whose first
// whitespace was a newline.
func TestQueryReadOnlyAcceptsMultiLineSelect(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	query := "select\n  status,\n  count(*) as n\nfrom products\ngroup by status"
	if _, err := db.QueryReadOnly(ctx, query); err != nil {
		t.Fatalf("multi-line select was rejected: %v", err)
	}

	// Widening the first-token check must not let a write through on a newline.
	if _, err := db.QueryReadOnly(ctx, "delete\n  from products"); err == nil {
		t.Fatal("multi-line delete was accepted")
	}
}

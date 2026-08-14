package appsumo_test

import (
	"testing"

	"github.com/vecyang1/appsumo-cli/internal/appsumo"
)

// liveShapedAccount reproduces what a real account returns: is_redeemed false on
// every row, including the products the buyer has plainly redeemed, plus the two
// AppSumo Plus subscription rows that legitimately carry no redeem_date.
func liveShapedAccount() []appsumo.Product {
	return []appsumo.Product{
		{Name: "Growify", Slug: "growify", Status: "activated", PlanName: "Tier 1",
			PurchaseDate: "2025-11-24", RedeemDate: "2025-11-25", IsRedeemed: false,
			HasActiveLicense: true, CanTransferLicense: true, IsRefundable: false},
		{Name: "Buildfast", Slug: "buildfast", Status: "expired", PlanName: "Tier 2",
			PurchaseDate: "2023-05-05", RedeemDate: "2023-05-06", IsRedeemed: false},
		{Name: "AppSumo Plus Yearly Plan", Status: "expired",
			PurchaseDate: "2023-05-05", RedeemDate: nil, IsRedeemed: false},
		{Name: "Neverused", Slug: "neverused", Status: "activated", PlanName: "Tier 1",
			PurchaseDate: "2026-08-01", RedeemDate: nil, IsRedeemed: false},
	}
}

// TestSummariseDerivesRedemptionFromDateNotFlag is the guard for the defect that
// makes this rollup worth having: is_redeemed reads false on all 70 products of
// a live account, including all 36 the buyer activated. Keying on it reports an
// action list of 70 items that do not exist.
func TestSummariseDerivesRedemptionFromDateNotFlag(t *testing.T) {
	summary := appsumo.Summarise(liveShapedAccount())

	if summary.Products != 4 {
		t.Fatalf("counted %d products, want 4", summary.Products)
	}
	if got := summary.ByRedemption[string(appsumo.Redeemed)]; got != 2 {
		t.Fatalf("redeemed = %d, want 2 (the two carrying a redeem_date)", got)
	}
	if got := summary.ByRedemption[string(appsumo.NotRedeemable)]; got != 1 {
		t.Fatalf("not_redeemable = %d, want 1 (the AppSumo Plus subscription)", got)
	}
	if got := summary.ByRedemption[string(appsumo.AwaitingRedeem)]; got != 1 {
		t.Fatalf("awaiting_redemption = %d, want 1", got)
	}

	// The action list is the number a buyer would act on. Reading is_redeemed
	// would have put all four rows here.
	if len(summary.AwaitingRedemption) != 1 || summary.AwaitingRedemption[0].Slug != "neverused" {
		t.Fatalf("action list wrong: %#v", summary.AwaitingRedemption)
	}
	if !hasWarningContaining(summary.Warnings, "is_redeemed is false on 2 products") {
		t.Fatalf("the flag/date disagreement was not reported: %v", summary.Warnings)
	}
}

// An expired product cannot still be redeemed, and a membership subscription was
// never redeemable. Neither belongs on a to-do list.
func TestSummariseKeepsUnredeemableProductsOffTheActionList(t *testing.T) {
	summary := appsumo.Summarise([]appsumo.Product{
		{Name: "Long Gone", Slug: "gone", Status: "expired", RedeemDate: nil},
		{Name: "AppSumo Plus Yearly Plan", Status: "activated", RedeemDate: nil},
	})
	if len(summary.AwaitingRedemption) != 0 {
		t.Fatalf("unredeemable products reached the action list: %#v", summary.AwaitingRedemption)
	}
}

// A healthy account emits no warnings. Without this the warning is untestable as
// a signal: one that fires on every input is noise a reader learns to skip.
func TestSummariseIsSilentWhenTheFlagAgrees(t *testing.T) {
	summary := appsumo.Summarise([]appsumo.Product{
		{Name: "Growify", Slug: "growify", Status: "activated",
			RedeemDate: "2025-11-25", IsRedeemed: true},
		{Name: "Neverused", Slug: "neverused", Status: "activated",
			RedeemDate: nil, IsRedeemed: false},
	})
	if len(summary.Warnings) != 0 {
		t.Fatalf("healthy account emitted warnings: %v", summary.Warnings)
	}
}

// If AppSumo ever starts populating is_redeemed, the disagreement must surface
// rather than being silently outvoted by the derivation.
func TestSummariseReportsDriftInBothDirections(t *testing.T) {
	summary := appsumo.Summarise([]appsumo.Product{
		{Name: "Flagged", Slug: "flagged", Status: "activated",
			RedeemDate: nil, IsRedeemed: true},
	})
	if !hasWarningContaining(summary.Warnings, "is_redeemed is true on 1 products with no redeem_date") {
		t.Fatalf("reverse drift was not reported: %v", summary.Warnings)
	}
}

func TestSummariseCountsLicensingAndRefundWindow(t *testing.T) {
	summary := appsumo.Summarise(liveShapedAccount())
	if summary.ActiveLicenses != 1 {
		t.Fatalf("active licenses = %d, want 1", summary.ActiveLicenses)
	}
	if summary.Transferable != 1 {
		t.Fatalf("transferable = %d, want 1", summary.Transferable)
	}
	if summary.Refundable != 0 {
		t.Fatalf("refundable = %d, want 0", summary.Refundable)
	}
	if summary.OldestPurchase != "2023-05-05" || summary.NewestPurchase != "2026-08-01" {
		t.Fatalf("purchase range wrong: %s to %s", summary.OldestPurchase, summary.NewestPurchase)
	}
}

// AppSumo encodes these flags as a bool on one endpoint and a string or number
// on another. A rollup that only understands one of them undercounts silently.
func TestSummariseReadsFlagsInEveryEncodingAppSumoUses(t *testing.T) {
	summary := appsumo.Summarise([]appsumo.Product{
		{Name: "A", Status: "activated", RedeemDate: "2025-01-01", IsRedeemed: true, CanTransferLicense: true},
		{Name: "B", Status: "activated", RedeemDate: "2025-01-01", IsRedeemed: "True", CanTransferLicense: "True"},
		{Name: "C", Status: "activated", RedeemDate: "2025-01-01", IsRedeemed: float64(1), CanTransferLicense: float64(1)},
		// "False" must read as false, so this row carries no redeem_date either;
		// otherwise the flag and the date genuinely disagree and the warning is
		// correct rather than spurious.
		{Name: "D", Status: "activated", RedeemDate: nil, IsRedeemed: "False", CanTransferLicense: "False"},
	})
	if summary.Transferable != 3 {
		t.Fatalf("transferable = %d, want 3; a flag encoding was not understood", summary.Transferable)
	}
	if len(summary.Warnings) != 0 {
		t.Fatalf("all four rows agree with their redeem_date but warnings fired: %v", summary.Warnings)
	}
}

func TestSummariseSaysWhenThereIsNothingToSummarise(t *testing.T) {
	summary := appsumo.Summarise(nil)
	if !hasWarningContaining(summary.Warnings, "run `appsumo sync` first") {
		t.Fatalf("an empty account gave no remedy: %v", summary.Warnings)
	}
}

package appsumo

// Account portfolio rollup.
//
// # is_redeemed is not a redemption signal
//
// Measured across all 70 products on a live account, 2026-08-14:
//
//	status              activated 36, expired 33, inactive 1
//	is_redeemed         false on 70 of 70
//	redeem_date present 68 of 70
//	activated with a redeem_date and is_redeemed false   36 of 36
//
// Every product the buyer has actually redeemed reports is_redeemed false. A
// rollup keyed on that flag reports 70 unredeemed products and reads as a
// to-do list of 70 items that do not exist — a specific, plausible, wrong
// answer, which is worse than a crash because the user acts on it.
//
// Redemption is therefore derived from redeem_date, which is populated on 68 of
// 70. The two exceptions are both "AppSumo Plus Yearly Plan" rows: subscriptions
// rather than redeemable licenses, so an absent redeem_date is correct there and
// NotRedeemable keeps them out of the action list instead of counting them as
// work.
//
// Summarise emits a warning whenever is_redeemed disagrees with redeem_date, so
// that if AppSumo ever starts populating the flag the drift is visible rather
// than assumed. Healthy input emits nothing.

import (
	"fmt"
	"sort"
	"strings"
)

// RedemptionState is a three-state answer: a product is redeemed, awaiting
// redemption, or not the kind of thing that gets redeemed at all.
type RedemptionState string

const (
	Redeemed       RedemptionState = "redeemed"
	AwaitingRedeem RedemptionState = "awaiting_redemption"
	NotRedeemable  RedemptionState = "not_redeemable"
)

// PortfolioSummary is an account rollup with its own caveats attached.
type PortfolioSummary struct {
	Products int `json:"products"`

	ByStatus      map[string]int `json:"by_status"`
	ByRedemption  map[string]int `json:"by_redemption"`
	ByListingPlan map[string]int `json:"by_plan"`

	ActiveLicenses int `json:"active_licenses"`
	Transferable   int `json:"transferable"`
	Refundable     int `json:"refundable"`

	OldestPurchase string `json:"oldest_purchase"`
	NewestPurchase string `json:"newest_purchase"`

	// AwaitingRedemption lists the products that actually need the buyer to do
	// something. It is deliberately derived, not read from is_redeemed.
	AwaitingRedemption []PortfolioItem `json:"awaiting_redemption"`

	// Warnings describe why a number might be a floor rather than a fact.
	Warnings []string `json:"warnings"`
}

// PortfolioItem names one product in an action list.
type PortfolioItem struct {
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Status   string `json:"status"`
	PlanName string `json:"plan_name"`
	Purchase string `json:"purchase_date"`
}

// Redemption classifies one product.
//
// A redeem_date is proof of redemption. Absent it, an expired or inactive
// product is past the point of being redeemable, and a subscription plan was
// never redeemable in the first place — neither belongs on an action list.
func (p Product) Redemption() RedemptionState {
	if strings.TrimSpace(stringOf(p.RedeemDate)) != "" {
		return Redeemed
	}
	if isSubscriptionPlan(p.Name) {
		return NotRedeemable
	}
	switch strings.ToLower(strings.TrimSpace(p.Status)) {
	case "expired", "refunded":
		return NotRedeemable
	}
	return AwaitingRedeem
}

// isSubscriptionPlan recognises AppSumo's own membership rows, which appear in
// the products list but are billing subscriptions, not licenses to redeem.
func isSubscriptionPlan(name string) bool {
	lowered := strings.ToLower(name)
	return strings.Contains(lowered, "appsumo plus") || strings.Contains(lowered, "appsumo select")
}

// Summarise rolls up an account and reports what it could not verify.
func Summarise(products []Product) PortfolioSummary {
	summary := PortfolioSummary{
		Products:      len(products),
		ByStatus:      map[string]int{},
		ByRedemption:  map[string]int{},
		ByListingPlan: map[string]int{},
	}

	datedButFlagFalse := 0
	flaggedWithoutDate := 0
	for _, product := range products {
		status := strings.TrimSpace(product.Status)
		if status == "" {
			status = "unknown"
		}
		summary.ByStatus[status]++

		state := product.Redemption()
		summary.ByRedemption[string(state)]++
		if state == AwaitingRedeem {
			summary.AwaitingRedemption = append(summary.AwaitingRedemption, PortfolioItem{
				Name: product.Name, Slug: product.Slug, Status: status,
				PlanName: product.PlanName, Purchase: product.PurchaseDate,
			})
		}

		if plan := strings.TrimSpace(product.PlanName); plan != "" {
			summary.ByListingPlan[plan]++
		}
		if boolOf(product.HasActiveLicense) {
			summary.ActiveLicenses++
		}
		if boolOf(product.CanTransferLicense) {
			summary.Transferable++
		}
		if boolOf(product.IsRefundable) {
			summary.Refundable++
		}
		switch {
		case boolOf(product.IsRedeemed) && state != Redeemed:
			flaggedWithoutDate++
		case !boolOf(product.IsRedeemed) && state == Redeemed:
			datedButFlagFalse++
		}

		if date := strings.TrimSpace(product.PurchaseDate); date != "" {
			if summary.OldestPurchase == "" || date < summary.OldestPurchase {
				summary.OldestPurchase = date
			}
			if date > summary.NewestPurchase {
				summary.NewestPurchase = date
			}
		}
	}

	sort.Slice(summary.AwaitingRedemption, func(i, j int) bool {
		return summary.AwaitingRedemption[i].Purchase > summary.AwaitingRedemption[j].Purchase
	})

	// Both directions are reported, because they mean different things. The
	// first is today's known defect. The second would mean AppSumo started
	// populating the flag, which is the moment this derivation needs rereading.
	if datedButFlagFalse > 0 {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf(
			"is_redeemed is false on %d products that carry a redeem_date; the flag is not a redemption signal and was not used",
			datedButFlagFalse))
	}
	if flaggedWithoutDate > 0 {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf(
			"is_redeemed is true on %d products with no redeem_date; the flag and the date now disagree in both directions",
			flaggedWithoutDate))
	}
	if len(products) == 0 {
		summary.Warnings = append(summary.Warnings,
			"no synced products; run `appsumo sync` first")
	}
	return summary
}

func stringOf(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

// boolOf reads a JSON field that AppSumo encodes as a bool, a number, or a
// string depending on the endpoint. An unreadable value is false, which is safe
// here only because every caller treats false as "did not state true" and never
// as evidence for the negative.
func boolOf(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes":
			return true
		}
		return false
	default:
		return false
	}
}

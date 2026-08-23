package appsumo

type Product struct {
	ID                        any    `json:"id"`
	UUID                      string `json:"uuid"`
	InvoiceUUID               string `json:"invoice_uuid"`
	Image                     string `json:"image"`
	Name                      string `json:"name"`
	Slug                      string `json:"slug"`
	Status                    string `json:"status"`
	PlanName                  string `json:"plan_name"`
	PlanID                    any    `json:"plan_id"`
	PlanHelpText              string `json:"plan_help_text"`
	RedeemText                string `json:"redeem_text"`
	IsRefunded                any    `json:"is_refunded"`
	DateRefunded              any    `json:"date_refunded"`
	IsRedeemed                any    `json:"is_redeemed"`
	IsBundle                  any    `json:"is_bundle"`
	HasActiveLicense          any    `json:"has_active_license"`
	CanActivateLicense        any    `json:"can_activate_license"`
	CanEnhanceOrReduceLicense any    `json:"can_enhance_or_reduce_license"`
	CanTransferLicense        any    `json:"can_transfer_license"`
	HasBeenTransferred        any    `json:"has_been_transferred"`
	IsRefundable              any    `json:"is_refundable"`
	ShowRefundLicenseOption   any    `json:"show_refund_license_option"`
	UseLicensing              any    `json:"use_licensing"`
	PurchaseDate              string `json:"purchase_date"`
	RedeemDate                any    `json:"redeem_date"`
	PlanStackCount            any    `json:"plan_stack_count"`
	LicenseV2Status           string `json:"license_v2_status"`
	Webinar                   any    `json:"webinar"`
	IsPlusProduct             any    `json:"is_plus_product"`
	SupportEmail              string `json:"support_email"`
	TransferDate              any    `json:"transfer_date"`
	AddonDeals                any    `json:"addon_deals"`
}

type ProductsEnvelope struct {
	Products                  ProductsPage   `json:"products"`
	AlternativeSKURedemptions map[string]any `json:"alternative_sku_redemptions"`
}

type ProductsPage struct {
	Count    int       `json:"count"`
	Next     *string   `json:"next"`
	Previous *string   `json:"previous"`
	Results  []Product `json:"results"`
}

type AuthStatus struct {
	HasCookie     bool `json:"has_cookie"`
	Authenticated bool `json:"authenticated"`

	Email      string `json:"-"`
	CustomerID string `json:"-"`
}

# AppSumo Account API Discovery

Discovery date: 2026-05-23

## Conclusion

AppSumo buyer-account data is exposed through private, session-cookie-backed web endpoints. The public AppSumo API documentation is partner/licensing oriented and is not a buyer OAuth API for account products.

## Verified Logged-In Buyer Endpoints

All endpoints below require the user's logged-in AppSumo browser session:

- `GET /api/v2/account/products/?page=1`
- `GET /api/v2/account/products/download/?`
- `GET /api/sessions/current/`
- `GET /api/sumopulse/notifications/unread-count/`

The products endpoint returns:

- Top-level keys: `products`, `alternative_sku_redemptions`
- `products` keys: `count`, `next`, `previous`, `results`
- First live check returned a private non-zero buyer product count. Exact account counts are intentionally omitted from tracked docs.

Product records include these stable fields:

- `id`
- `uuid`
- `invoice_uuid`
- `image`
- `name`
- `slug`
- `status`
- `plan_name`
- `plan_id`
- `plan_help_text`
- `redeem_text`
- `is_refunded`
- `date_refunded`
- `is_redeemed`
- `is_bundle`
- `has_active_license`
- `can_activate_license`
- `can_enhance_or_reduce_license`
- `can_transfer_license`
- `has_been_transferred`
- `is_refundable`
- `show_refund_license_option`
- `use_licensing`
- `purchase_date`
- `redeem_date`
- `plan_stack_count`
- `license_v2_status`
- `webinar`
- `is_plus_product`
- `support_email`
- `transfer_date`
- `addon_deals`

Live data can encode `id` as either a JSON number or string, so the CLI treats it as an opaque value rather than an arithmetic integer.

The CSV export endpoint returns `text/csv` with a `License Key / Code` column. That column is sensitive and must always be redacted by the public CLI surface.

Redemption pages expose richer data through `__NEXT_DATA__`, including license fields such as `invoice_item_license_key`. Those fields are sensitive and are not part of the v1 read-only MVP output.

## Public Documentation Checked

- AppSumo Licensing API overview: `https://docs.licensing.appsumo.com/api/api__overview.html`
- AppSumo Licensing API getting started: `https://docs.licensing.appsumo.com/api/api__getting_started.html`
- Purchase history help: `https://help.appsumo.com/article/29-purchase-history`
- Invoices help: `https://help.appsumo.com/article/30-invoices`
- Refund request help: `https://help.appsumo.com/article/33-refund-request`
- License in account help: `https://help.appsumo.com/article/109-where-is-my-license-in-my-account`

## Safety Notes

- Do not store HAR files with cookies or license-code values.
- Do not print full session data from `/api/sessions/current/`; it includes personal and billing-shaped fields.
- Treat AppSumo's account UI actions as potentially mutating unless proven otherwise.

# Security

## Reporting

Please open a private security advisory on GitHub if you find a way for this CLI to leak cookies, account identifiers, billing data, or license codes.

## Privacy Model

The public CLI is intentionally read-only and redacts license/code fields. It only sends AppSumo cookies to `https://appsumo.com` or loopback test hosts.

Do not publish real AppSumo session cookies, HAR files, raw CSV exports, exact private product counts, customer IDs, account emails, billing fields, or license codes.

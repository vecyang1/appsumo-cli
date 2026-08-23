package redact

import (
	"bytes"
	"encoding/csv"
	"strings"
	"unicode"
)

const Marker = "[REDACTED]"

func CSV(input []byte) ([]byte, error) {
	reader := csv.NewReader(bytes.NewReader(input))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return input, nil
	}

	sensitive := make(map[int]struct{})
	for i, header := range rows[0] {
		if IsSensitiveKey(header) {
			sensitive[i] = struct{}{}
		}
	}
	for rowIndex := 1; rowIndex < len(rows); rowIndex++ {
		for col := range sensitive {
			if col < len(rows[rowIndex]) && rows[rowIndex][col] != "" {
				rows[rowIndex][col] = Marker
			}
		}
	}

	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	if err := writer.WriteAll(rows); err != nil {
		return nil, err
	}
	return out.Bytes(), writer.Error()
}

func JSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			if IsSensitiveKey(key) {
				out[key] = Marker
				continue
			}
			out[key] = JSONValue(child)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = JSONValue(child)
		}
		return out
	default:
		return value
	}
}

func IsSensitiveKey(key string) bool {
	normalized := normalize(key)
	switch normalized {
	case "cookie", "csrf", "csrftoken", "session", "sessionid", "token", "access_token", "refresh_token", "authorization", "customer_id", "email":
		return true
	case "license_key_code", "license_key", "license_code", "invoice_item_license_key", "invoice_item_bundle_codes":
		return true
	}
	return (strings.Contains(normalized, "license") && (strings.Contains(normalized, "key") || strings.Contains(normalized, "code"))) ||
		strings.Contains(normalized, "csrf") ||
		strings.Contains(normalized, "cookie")
}

func normalize(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range key {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

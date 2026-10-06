package appsumo

import (
	"fmt"
	"strconv"
	"strings"
)

// flexInt64 accepts an identifier encoded as either a JSON number or a string.
// AppSumo encodes ids both ways across this API, sometimes within one payload.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == `""` {
		*f = 0
		return nil
	}
	trimmed = strings.Trim(trimmed, `"`)
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return fmt.Errorf("decode id %s: %w", trimmed, err)
	}
	*f = flexInt64(parsed)
	return nil
}

// flexFloat64 accepts a money amount encoded as either a JSON number or a
// string. AppSumo's catalog returns "49.00" for some prices and 49 for others.
type flexFloat64 float64

func (f *flexFloat64) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == `""` {
		*f = 0
		return nil
	}
	trimmed = strings.Trim(trimmed, `"`)
	parsed, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return fmt.Errorf("decode amount %s: %w", trimmed, err)
	}
	*f = flexFloat64(parsed)
	return nil
}

// FlexBool accepts a boolean encoded as a JSON bool, number (0/1), or string
// (e.g. "true", "false", or a timestamp string like "2021-01-26T12:37:42...").
type FlexBool bool
type flexBool = FlexBool

func (f *FlexBool) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == `""` {
		*f = false
		return nil
	}
	if trimmed == "true" {
		*f = true
		return nil
	}
	if trimmed == "false" {
		*f = false
		return nil
	}
	if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		*f = (n != 0)
		return nil
	}
	unquoted := strings.Trim(trimmed, `"`)
	lower := strings.ToLower(unquoted)
	if lower == "true" || lower == "1" {
		*f = true
		return nil
	}
	if lower == "false" || lower == "0" || lower == "" {
		*f = false
		return nil
	}
	// Any other non-empty string (like an ISO timestamp "2021-01-26T...") indicates the flag is set (truthy).
	*f = true
	return nil
}

// FlexInt accepts an int encoded as either a JSON number or a string.
type FlexInt int
type flexInt = FlexInt

func (f *FlexInt) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == `""` {
		*f = 0
		return nil
	}
	trimmed = strings.Trim(trimmed, `"`)
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return fmt.Errorf("decode int %s: %w", trimmed, err)
	}
	*f = FlexInt(parsed)
	return nil
}

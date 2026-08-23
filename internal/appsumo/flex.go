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

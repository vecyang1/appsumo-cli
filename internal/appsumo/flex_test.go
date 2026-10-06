package appsumo

import (
	"encoding/json"
	"testing"
)

func TestFlexBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`true`, true},
		{`false`, false},
		{`"true"`, true},
		{`"false"`, false},
		{`"1"`, true},
		{`"0"`, false},
		{`1`, true},
		{`0`, false},
		{`"2021-01-26T12:37:42.136693-06:00"`, true}, // legacy timestamp in pinned field
		{`""`, false},
		{`null`, false},
	}

	for _, tt := range tests {
		var b flexBool
		if err := json.Unmarshal([]byte(tt.input), &b); err != nil {
			t.Errorf("Unmarshal %s failed: %v", tt.input, err)
		}
		if bool(b) != tt.expected {
			t.Errorf("Unmarshal %s = %v; want %v", tt.input, b, tt.expected)
		}
	}
}

func TestFlexInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{`123`, 123},
		{`"456"`, 456},
		{`""`, 0},
		{`null`, 0},
	}

	for _, tt := range tests {
		var i flexInt
		if err := json.Unmarshal([]byte(tt.input), &i); err != nil {
			t.Errorf("Unmarshal %s failed: %v", tt.input, err)
		}
		if int(i) != tt.expected {
			t.Errorf("Unmarshal %s = %v; want %v", tt.input, i, tt.expected)
		}
	}
}

package entity

import (
	"testing"
)

func TestJSONB_Value(t *testing.T) {
	tests := []struct {
		name     string
		input    JSONB
		expected string
		isNull   bool
	}{
		{
			name:   "nil slice",
			input:  nil,
			isNull: true,
		},
		{
			name:   "empty slice",
			input:  JSONB([]byte{}),
			isNull: true,
		},
		{
			name:     "valid json object",
			input:    JSONB([]byte(`{"status":"success"}`)),
			expected: `{"status":"success"}`,
		},
		{
			name:     "valid json array",
			input:    JSONB([]byte(`[1,2,3]`)),
			expected: `[1,2,3]`,
		},
		{
			name:     "strips null bytes from json",
			input:    JSONB([]byte("{\"message\":\"hello\x00world\"}")),
			expected: `{"message":"helloworld"}`,
		},
		{
			name:     "wraps plain text",
			input:    JSONB([]byte("Bad Gateway")),
			expected: `"Bad Gateway"`,
		},
		{
			name:     "strips null bytes and invalid utf8 from non-json binary",
			input:    JSONB([]byte("\x1f\x8b\x00plain")),
			expected: `"\u001fplain"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.input.Value()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.isNull {
				if val != nil {
					t.Fatalf("expected nil value, got %v", val)
				}
				return
			}
			b, ok := val.([]byte)
			if !ok {
				t.Fatalf("expected []byte value, got %T", val)
			}
			if string(b) != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, string(b))
			}
		})
	}
}

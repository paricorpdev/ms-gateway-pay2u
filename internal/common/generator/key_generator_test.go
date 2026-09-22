package generator

import (
	"strings"
	"testing"
)

func TestNormalizeCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Localoka V2", "localoka-v2"},
		{"LOCALOKA-V2", "localoka-v2"},
		{"  Order Service  ", "order-service"},
		{"POS / Outlet #01", "pos-outlet-01"},
		{"Already-Normalized_123", "already-normalized_123"},
		{"Multiple---Dashes", "multiple-dashes"},
	}

	for _, tt := range tests {
		got := NormalizeCode(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeCode(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey("Localoka V2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPrefix := "pg_localoka-v2_"
	if !strings.HasPrefix(key, expectedPrefix) {
		t.Fatalf("expected key to start with %q, got %q", expectedPrefix, key)
	}

	entropy := strings.TrimPrefix(key, expectedPrefix)
	if len(entropy) != 32 {
		t.Fatalf("expected 32 hex chars of entropy, got %d (%q)", len(entropy), entropy)
	}

	key2, _ := GenerateAPIKey("Localoka V2")
	if key == key2 {
		t.Fatalf("consecutive keys should have different entropy, got identical: %q", key)
	}

	if _, err := GenerateAPIKey("   "); err == nil {
		t.Fatal("expected error on empty code, got nil")
	}
}

func TestGenerateWebhookSecret(t *testing.T) {
	sec, err := GenerateWebhookSecret()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(sec, "whsec_") {
		t.Fatalf("expected secret to start with 'whsec_', got %q", sec)
	}

	entropy := strings.TrimPrefix(sec, "whsec_")
	if len(entropy) != 48 {
		t.Fatalf("expected 48 hex chars of entropy, got %d (%q)", len(entropy), entropy)
	}
}

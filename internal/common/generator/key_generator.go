package generator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var (
	nonAlphaNumericRegex = regexp.MustCompile(`[^a-z0-9\-_]+`)
	multipleDashesRegex  = regexp.MustCompile(`-+`)
)

func NormalizeCode(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	hyphenated := nonAlphaNumericRegex.ReplaceAllString(lower, "-")
	cleaned := multipleDashesRegex.ReplaceAllString(hyphenated, "-")
	return strings.Trim(cleaned, "-")
}

func GenerateAPIKey(code string) (string, error) {
	normCode := NormalizeCode(code)
	if normCode == "" {
		return "", fmt.Errorf("merchant code cannot be empty")
	}

	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("crypto rand failed: %w", err)
	}

	return fmt.Sprintf("pg_%s_%s", normCode, hex.EncodeToString(bytes)), nil
}

func GenerateWebhookSecret() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("crypto rand failed: %w", err)
	}

	return fmt.Sprintf("whsec_%s", hex.EncodeToString(bytes)), nil
}

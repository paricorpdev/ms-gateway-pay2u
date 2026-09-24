package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func ComputeSignature(payload []byte, secret string) string {
	if secret == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

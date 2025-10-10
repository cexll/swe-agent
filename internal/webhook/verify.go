package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// VerifySignature verifies the GitHub webhook signature
// using HMAC SHA-256 and constant-time comparison
func VerifySignature(payload []byte, signature, secret string) bool {
	// GitHub sends signature in format "sha256=<hash>"
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	// Extract the hash part
	receivedHash := strings.TrimPrefix(signature, "sha256=")

	// Compute expected hash
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(receivedHash), []byte(expectedHash))
}

// ValidateSignatureHeader validates the X-Hub-Signature-256 header
func ValidateSignatureHeader(header string) error {
	if header == "" {
		return fmt.Errorf("missing X-Hub-Signature-256 header")
	}
	if !strings.HasPrefix(header, "sha256=") {
		return fmt.Errorf("invalid signature format, expected 'sha256=<hash>'")
	}
	return nil
}

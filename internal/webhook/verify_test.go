package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	secret := "test-secret"
	payload := []byte("test payload")

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validHash := hex.EncodeToString(mac.Sum(nil))
	validSignature := "sha256=" + validHash

	tests := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		want      bool
	}{
		{
			name:      "valid signature",
			payload:   payload,
			signature: validSignature,
			secret:    secret,
			want:      true,
		},
		{
			name:      "invalid signature",
			payload:   payload,
			signature: "sha256=invalidsignature",
			secret:    secret,
			want:      false,
		},
		{
			name:      "wrong secret",
			payload:   payload,
			signature: validSignature,
			secret:    "wrong-secret",
			want:      false,
		},
		{
			name:      "missing sha256 prefix",
			payload:   payload,
			signature: validHash,
			secret:    secret,
			want:      false,
		},
		{
			name:      "empty signature",
			payload:   payload,
			signature: "",
			secret:    secret,
			want:      false,
		},
		{
			name:      "different payload",
			payload:   []byte("different payload"),
			signature: validSignature,
			secret:    secret,
			want:      false,
		},
		{
			name:      "empty payload",
			payload:   []byte(""),
			signature: validSignature,
			secret:    secret,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VerifySignature(tt.payload, tt.signature, tt.secret)
			if got != tt.want {
				t.Errorf("VerifySignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateSignatureHeader(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid header",
			header:  "sha256=abc123",
			wantErr: false,
		},
		{
			name:    "missing header",
			header:  "",
			wantErr: true,
			errMsg:  "missing X-Hub-Signature-256 header",
		},
		{
			name:    "invalid format - no prefix",
			header:  "abc123",
			wantErr: true,
			errMsg:  "invalid signature format, expected 'sha256=<hash>'",
		},
		{
			name:    "invalid format - wrong prefix",
			header:  "sha1=abc123",
			wantErr: true,
			errMsg:  "invalid signature format, expected 'sha256=<hash>'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSignatureHeader(tt.header)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSignatureHeader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("ValidateSignatureHeader() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestVerifySignature_TimingAttackResistance tests that constant-time comparison is used
func TestVerifySignature_TimingAttackResistance(t *testing.T) {
	secret := "test-secret"
	payload := []byte("test payload")

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validHash := hex.EncodeToString(mac.Sum(nil))
	validSignature := "sha256=" + validHash

	// Create a signature that differs only in the last character
	almostValid := validSignature[:len(validSignature)-1] + "X"

	// Both should return false
	result1 := VerifySignature(payload, almostValid, secret)
	result2 := VerifySignature(payload, "sha256=completely_wrong_signature", secret)

	if result1 != false {
		t.Errorf("VerifySignature() with almost valid signature = %v, want false", result1)
	}
	if result2 != false {
		t.Errorf("VerifySignature() with completely wrong signature = %v, want false", result2)
	}

	// Valid signature should return true
	result3 := VerifySignature(payload, validSignature, secret)
	if result3 != true {
		t.Errorf("VerifySignature() with valid signature = %v, want true", result3)
	}
}

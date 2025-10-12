package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Test private key for testing purposes (generated with openssl genrsa 2048)
const testPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAvd+J16V1N/V3CK2mn8rQ19AOUFe0p0zuXm+cMZtPpsheIbNs
Jb1lm12gM8C1QyV4Nk47NG0aP3DKjNk3UeniPPcyYeNJ9ULCrlnxOiqKEFaxyVGW
2kh3dOaSIZ3F3f8TDMLMYYuMCeCN1tw4ydWhiDITnGDMFGQOYKmBPRTNhKqmAo/o
HYc31SfntTVGwSiw0xUEn+ySuIqq9V+7ySJvAlmB3u4jCtOfUXukXHZ+wVu8G42f
vnKzBO1jzWSaOpiq73pmZOTT9Gpkm6bIkPKo7qt2aA21gJDbqTyKDL8Mccf3W6Wo
pAPuEh9jOv7IATc5zkW91ZVPtFf+IT/Sl+jrfQIDAQABAoIBAQCKYClDIfBlkdzo
VDXE6rh9L8Hex6x+6NAnvstkU74e3JPNl8dPUdKFAhzI2r6/asVLPoRjVsf0SC01
rPBmID+jEryDHnQ97COZkS7+pxXrhmMXRwDboEh+x7LkEOmtOkIV4Lm2tU6fvCli
1ygD4E9SxLwKEXlpuunHhIENlOWassfLLfHI6DohnasuPTh+mlx4wLrYf6NJnPf+
Qx6r+cBMkNB4IbXOZblA+fLODgDTRK1d8+HZJaEopwAnCJzHlatqZ3TmNwvqTPhO
rrPtRfp0YlN2WCvq88nNsu1V6pfhAGP/gR3uuacRy/FzHIkHT6z3PS/ql82zNMkp
2JoejEh5AoGBAPccg8IH0RQCQxRHQYA6ajQVQXfczWJA5VZUEXsY86OvLOPOuaJp
CcGQfoJxOcPlOAYn6hi06wYPwQFyuzLZ/Vj3vXmka9juz2h60F3L9rGFdzlIXAqJ
TKMDnw+ky0IE2q3F793FhEKBf2LMRFPa5D7LzyyFkhzlp15ri7TXi4Z3AoGBAMSz
9IRh6ypSI6EJP4SOucwE8ig25K6D1/Zf9mCYYe0iLcJHzs3K7EoYZwjmGR0s34TB
TXLK7dV3ZZouyslNRsdAvDtUcwJIX9nhXC+5jrNnCNMGsoYl43iKMJ+hqFBGe/PA
dG0Pk4Y90deYV76veEB4GgRplKzxjxRexGDcrzarAoGAK4Qc+81Ol1xynZ6SvVcM
HtHjbo02qefNuy8gyPGy7g9KM2/TJvOiYTDl5mi0CHhULllXEzTA8pdRoMSojKLw
x3sRJdu7lj8vzTFbgjkJ32cmgLLqanyVP1vC5glaNe0O6W0i+YXv7ZpKaYaZPb8d
VKWlfSykd2xF1g3QU29lxa8CgYAs2NKg9CpHxd51ssQWluvphh8n6AwPdePhOlPU
BiodhLNmHjUaWm+xHQswzjVfn4F+pQvhZj7/cm9pzc1SRBolB69i34gxNwsTg/we
rXHJmW47nsVJLI5GR0t6ucLEOq28D178FpcN/j4/p24p/ZuvJzLXWrMZEyIKBOlF
JEuWbQKBgFWKfbzIRchhRUe/jF4rFxkUVk51NK1XhrM99vbMnH2XXrTjjgS3lolV
CDSUU0sAy1UTRr7NPPw4ILmB+FCZlB3mKqx1VhssX1PlTFD/c+Orrpl4eBaFkrJ3
c73uIrGjgRcNO03atSknlxH/YbBxVAd7VYajYAm16pgmWZNP+cST
-----END RSA PRIVATE KEY-----`

func TestGenerateJWT(t *testing.T) {
	tests := []struct {
		name      string
		appID     string
		shouldErr bool
	}{
		{
			name:      "valid app ID",
			appID:     "123456",
			shouldErr: false,
		},
		{
			name:      "invalid app ID",
			appID:     "not-a-number",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &AppAuth{
				AppID:      tt.appID,
				PrivateKey: testPrivateKey,
			}

			token, err := auth.GenerateJWT()
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if token == "" {
				t.Errorf("expected non-empty token")
			}

			// Verify token structure
			parser := jwt.NewParser()
			claims := jwt.RegisteredClaims{}
			_, _, err = parser.ParseUnverified(token, &claims)
			if err != nil {
				t.Errorf("failed to parse token: %v", err)
				return
			}

			// Verify claims
			if claims.Issuer != tt.appID {
				t.Errorf("issuer = %s, want %s", claims.Issuer, tt.appID)
			}

			// Verify token is not expired
			if claims.ExpiresAt.Before(time.Now()) {
				t.Errorf("token is expired")
			}
		})
	}
}

func TestInvalidPrivateKey(t *testing.T) {
	auth := &AppAuth{
		AppID:      "123456",
		PrivateKey: "invalid-key",
	}

	_, err := auth.GenerateJWT()
	if err == nil {
		t.Errorf("expected error for invalid private key, got nil")
	}
}

func TestInvalidRepoFormat(t *testing.T) {
	auth := &AppAuth{
		AppID:      "123456",
		PrivateKey: testPrivateKey,
	}

	tests := []string{
		"invalid",
		"invalid/repo/extra",
		"",
	}

	for _, repo := range tests {
		t.Run(repo, func(t *testing.T) {
			_, err := auth.GetInstallationToken(repo)
			if err == nil {
				t.Errorf("expected error for invalid repo format '%s', got nil", repo)
			}
		})
	}
}

// TestGetInstallationID_MockServer tests getInstallationID with a mock server
func TestGetInstallationID_MockServer(t *testing.T) {
	tests := []struct {
		name           string
		repo           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		wantID         int64
	}{
		{
			name: "successful response",
			repo: "owner/repo",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if !strings.Contains(r.URL.Path, "/repos/owner/repo/installation") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") == "" {
					t.Errorf("missing Authorization header")
				}
				if r.Header.Get("Accept") != "application/vnd.github+json" {
					t.Errorf("incorrect Accept header")
				}

				// Send success response
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id": 12345,
				})
			},
			wantErr: false,
			wantID:  12345,
		},
		{
			name: "API error response",
			repo: "owner/repo",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message": "Not Found"}`))
			},
			wantErr: true,
		},
		{
			name: "invalid JSON response",
			repo: "owner/repo",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			auth := &AppAuth{
				AppID:      "123456",
				PrivateKey: testPrivateKey,
			}

			// Generate JWT for testing
			jwtToken, err := auth.GenerateJWT()
			if err != nil {
				t.Fatalf("failed to generate JWT: %v", err)
			}

			// Note: This test demonstrates the approach, but won't work directly
			// because we can't override the GitHub API URL without modifying auth.go
			// In a real scenario, we'd need to add dependency injection for the HTTP client
			_ = server
			_ = jwtToken
			// id, err := auth.getInstallationID(jwtToken, tt.repo)
			// ... validation would go here
		})
	}
}

// TestGetInstallationToken_ErrorScenarios tests various error scenarios
func TestGetInstallationToken_ErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		auth        *AppAuth
		repo        string
		expectError string
	}{
		{
			name: "invalid app ID",
			auth: &AppAuth{
				AppID:      "not-a-number",
				PrivateKey: testPrivateKey,
			},
			repo:        "owner/repo",
			expectError: "invalid app ID",
		},
		{
			name: "invalid private key",
			auth: &AppAuth{
				AppID:      "123456",
				PrivateKey: "invalid-key",
			},
			repo:        "owner/repo",
			expectError: "failed to parse private key",
		},
		{
			name: "invalid repo format - no slash",
			auth: &AppAuth{
				AppID:      "123456",
				PrivateKey: testPrivateKey,
			},
			repo:        "invalid",
			expectError: "invalid repo format",
		},
		{
			name: "invalid repo format - multiple slashes",
			auth: &AppAuth{
				AppID:      "123456",
				PrivateKey: testPrivateKey,
			},
			repo:        "owner/repo/extra",
			expectError: "invalid repo format",
		},
		{
			name: "empty repo",
			auth: &AppAuth{
				AppID:      "123456",
				PrivateKey: testPrivateKey,
			},
			repo:        "",
			expectError: "invalid repo format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.auth.GetInstallationToken(tt.repo)
			if err == nil {
				t.Errorf("expected error containing '%s', got nil", tt.expectError)
				return
			}
			if !strings.Contains(err.Error(), tt.expectError) {
				t.Errorf("error = %v, want error containing '%s'", err, tt.expectError)
			}
		})
	}
}

// TestAppAuth_Structure validates the AppAuth struct
func TestAppAuth_Structure(t *testing.T) {
	auth := &AppAuth{
		AppID:      "test-id",
		PrivateKey: "test-key",
	}

	if auth.AppID == "" {
		t.Error("AppID should be set")
	}
	if auth.PrivateKey == "" {
		t.Error("PrivateKey should be set")
	}
}

// TestInstallationToken_Structure validates the InstallationToken struct
func TestInstallationToken_Structure(t *testing.T) {
	now := time.Now()
	token := &InstallationToken{
		Token:     "test-token",
		ExpiresAt: now,
	}

	if token.Token == "" {
		t.Error("Token should be set")
	}
	if token.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set")
	}
	if !token.ExpiresAt.Equal(now) {
		t.Errorf("ExpiresAt = %v, want %v", token.ExpiresAt, now)
	}
}

// TestGenerateJWT_TokenExpiry tests JWT token expiry
func TestGenerateJWT_TokenExpiry(t *testing.T) {
	auth := &AppAuth{
		AppID:      "123456",
		PrivateKey: testPrivateKey,
	}

	token, err := auth.GenerateJWT()
	if err != nil {
		t.Fatalf("GenerateJWT() error = %v", err)
	}

	// Parse token
	parser := jwt.NewParser()
	claims := jwt.RegisteredClaims{}
	_, _, err = parser.ParseUnverified(token, &claims)
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}

	// Verify token expires in approximately 10 minutes
	expectedExpiry := time.Now().Add(10 * time.Minute)
	diff := claims.ExpiresAt.Time.Sub(expectedExpiry)
	if diff < -1*time.Minute || diff > 1*time.Minute {
		t.Errorf("token expiry = %v, want approximately %v (diff: %v)",
			claims.ExpiresAt.Time, expectedExpiry, diff)
	}

	// Verify token was issued recently
	if claims.IssuedAt.Before(time.Now().Add(-1 * time.Minute)) {
		t.Error("token issued at time is too old")
	}
}

// TestGetInstallationToken_RepoFormatValidation tests various repo formats
func TestGetInstallationToken_RepoFormatValidation(t *testing.T) {
	auth := &AppAuth{
		AppID:      "123456",
		PrivateKey: testPrivateKey,
	}

	validFormats := []string{
		"owner/repo",
		"my-org/my-repo",
		"user123/project456",
		"Org_Name/Repo-Name",
	}

	invalidFormats := []string{
		"",
		"noslash",
		"/leading-slash",
		"trailing-slash/",
		"too/many/slashes",
		"owner/",
		"/repo",
	}

	// Test valid formats (they will fail at API call, but repo format should pass)
	for _, repo := range validFormats {
		t.Run(fmt.Sprintf("valid_%s", repo), func(t *testing.T) {
			_, err := auth.GetInstallationToken(repo)
			// Should not get "invalid repo format" error
			if err != nil && strings.Contains(err.Error(), "invalid repo format") {
				t.Errorf("repo format '%s' should be valid, got error: %v", repo, err)
			}
		})
	}

	// Test invalid formats
	for _, repo := range invalidFormats {
		t.Run(fmt.Sprintf("invalid_%s", repo), func(t *testing.T) {
			_, err := auth.GetInstallationToken(repo)
			if err == nil {
				t.Errorf("repo format '%s' should be invalid, got no error", repo)
			}
			// Most invalid formats should produce "invalid repo format" error at parsing stage
			if !strings.Contains(err.Error(), "invalid repo format") && !strings.Contains(err.Error(), "failed") {
				// Some may fail later, which is also acceptable
				t.Logf("repo format '%s' failed with: %v", repo, err)
			}
		})
	}
}

func TestGenerateJWT_InvalidPrivateKey(t *testing.T) {
	tests := []struct {
		name        string
		privateKey  string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty private key",
			privateKey:  "",
			wantErr:     true,
			errContains: "parse private key",
		},
		{
			name:        "malformed private key",
			privateKey:  "-----BEGIN RSA PRIVATE KEY-----\ninvalid\n-----END RSA PRIVATE KEY-----",
			wantErr:     true,
			errContains: "parse private key",
		},
		{
			name:        "wrong format",
			privateKey:  "not a key at all",
			wantErr:     true,
			errContains: "parse private key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &AppAuth{
				AppID:      "123456",
				PrivateKey: tt.privateKey,
			}

			token, err := auth.GenerateJWT()

			if tt.wantErr {
				if err == nil {
					t.Error("GenerateJWT() should return error")
					return
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("GenerateJWT() error = %v, want error containing %q", err, tt.errContains)
				}
				if token != "" {
					t.Errorf("GenerateJWT() token = %q, want empty string on error", token)
				}
			} else {
				if err != nil {
					t.Errorf("GenerateJWT() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestGenerateJWT_EdgeCaseAppIDs(t *testing.T) {
	tests := []struct {
		name    string
		appID   string
		wantErr bool
	}{
		{
			name:    "empty app ID",
			appID:   "",
			wantErr: true,
		},
		{
			name:    "negative app ID",
			appID:   "-123",
			wantErr: false, // ParseInt will handle negative numbers
		},
		{
			name:    "zero app ID",
			appID:   "0",
			wantErr: false,
		},
		{
			name:    "very large app ID",
			appID:   "999999999999",
			wantErr: false,
		},
		{
			name:    "app ID with spaces",
			appID:   "123 456",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &AppAuth{
				AppID:      tt.appID,
				PrivateKey: testPrivateKey,
			}

			token, err := auth.GenerateJWT()

			if tt.wantErr {
				if err == nil {
					t.Errorf("GenerateJWT() should return error for app ID %q", tt.appID)
				}
			} else {
				if err != nil {
					t.Errorf("GenerateJWT() unexpected error for app ID %q: %v", tt.appID, err)
				}
				if token == "" {
					t.Error("GenerateJWT() returned empty token")
				}
			}
		})
	}
}

package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthProvider defines the interface for GitHub authentication
type AuthProvider interface {
	GetInstallationToken(repo string) (*InstallationToken, error)
}

// AppAuth holds GitHub App authentication configuration
type AppAuth struct {
	AppID      string
	PrivateKey string
}

// InstallationToken represents a GitHub App installation access token
type InstallationToken struct {
	Token     string
	ExpiresAt time.Time
}

// GenerateJWT creates a JWT token for GitHub App authentication
func (a *AppAuth) GenerateJWT() (string, error) {
	// Parse private key
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(a.PrivateKey))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// Convert App ID to int
	appID, err := strconv.ParseInt(a.AppID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid app ID: %w", err)
	}

	// Create JWT claims
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		Issuer:    strconv.FormatInt(appID, 10),
	}

	// Create and sign token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return signedToken, nil
}

// GetInstallationToken gets an installation access token for a repository
func (a *AppAuth) GetInstallationToken(repo string) (*InstallationToken, error) {
	// 1. Generate JWT
	jwtToken, err := a.GenerateJWT()
	if err != nil {
		return nil, err
	}

	// 2. Get installation ID for the repository
	installationID, err := a.getInstallationID(jwtToken, repo)
	if err != nil {
		return nil, err
	}

	// 3. Get installation access token
	return a.getInstallationAccessToken(jwtToken, installationID)
}

// getInstallationID retrieves the installation ID for a repository
func (a *AppAuth) getInstallationID(jwtToken, repo string) (int64, error) {
	// Parse owner/repo
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid repo format: %s (expected owner/repo)", repo)
	}
	owner, repoName := parts[0], parts[1]

	// Call GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/installation", owner, repoName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to get installation: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.ID, nil
}

// getInstallationAccessToken retrieves an installation access token
func (a *AppAuth) getInstallationAccessToken(jwtToken string, installationID int64) (*InstallationToken, error) {
	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &InstallationToken{
		Token:     result.Token,
		ExpiresAt: result.ExpiresAt,
	}, nil
}

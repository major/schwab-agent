package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/major/schwab-agent/internal/apperr"
)

const (
	// refreshTokenMaxAge is 6.5 days in seconds, matching schwab-py's default.
	refreshTokenMaxAge = 561600

	// accessTokenLeeway is 5 minutes in seconds, matching schwab-py's leeway.
	accessTokenLeeway = 300
)

// TokenFile is the on-disk format matching schwab-py.
type TokenFile struct {
	CreationTimestamp int64     `json:"creation_timestamp"`
	Token             TokenData `json:"token"`
}

// TokenData holds the OAuth token fields.
type TokenData struct {
	AccessToken  string  `json:"access_token"`
	TokenType    string  `json:"token_type"`
	ExpiresIn    int     `json:"expires_in"`
	RefreshToken string  `json:"refresh_token"`
	Scope        string  `json:"scope"`
	ExpiresAt    float64 `json:"expires_at"`
}

// LoadToken reads and parses the token file.
// Returns AuthRequiredError if the file doesn't exist.
func LoadToken(tokenPath string) (*TokenFile, error) {
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.NewAuthRequiredError(
				"token file not found: run `schwab-agent auth login` to authenticate",
				err,
			)
		}
		return nil, fmt.Errorf("failed to read token file: %w", err)
	}

	var tf TokenFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("failed to parse token file: %w", err)
	}

	return &tf, nil
}

// SaveToken writes the token file with 0600 permissions.
// Creates the parent directory with 0700 if missing.
func SaveToken(tokenPath string, tf *TokenFile) error {
	parentDir := filepath.Dir(tokenPath)
	if err := os.MkdirAll(parentDir, 0o700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	data, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}

	if err := os.WriteFile(tokenPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write token file: %w", err)
	}

	return nil
}

// IsAccessTokenExpired checks if the access token is expired (with 300s leeway).
// Returns true if time.Now() >= ExpiresAt - 300.
func IsAccessTokenExpired(tf *TokenFile) bool {
	return float64(time.Now().Unix()) >= tf.Token.ExpiresAt-accessTokenLeeway
}

// IsRefreshTokenStale checks if the refresh token is stale (> 6.5 days old).
// Returns true if time.Now().Unix() >= creation_timestamp + 561600.
func IsRefreshTokenStale(tf *TokenFile) bool {
	return time.Now().Unix() >= tf.CreationTimestamp+refreshTokenMaxAge
}

// tokenErrorResponse represents the error body from the token endpoint.
type tokenErrorResponse struct {
	Error string `json:"error"`
}

// RefreshAccessToken exchanges the refresh token for a new access token.
// Preserves creation_timestamp from the original TokenFile.
// Returns AuthExpiredError if the refresh fails with invalid_grant.
// The endpoint parameter allows overriding the token URL for testing.
func RefreshAccessToken(cfg *Config, tf *TokenFile, endpoint string) (*TokenFile, error) {
	if endpoint == "" {
		endpoint = cfg.OAuthTokenURL()
	}

	client := newOAuthClient(cfg, oauthHTTPTimeout)
	defer client.Close()

	resp, err := client.R().
		SetBasicAuth(cfg.ClientID, cfg.ClientSecret).
		SetFormData(map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": tf.Token.RefreshToken,
		}).
		Post(endpoint)
	if err != nil {
		return nil, fmt.Errorf("token refresh request failed: %w", err)
	}

	// Handle non-2xx responses
	if resp.StatusCode() != 200 {
		var errResp tokenErrorResponse
		if json.Unmarshal(resp.Bytes(), &errResp) == nil && errResp.Error == "invalid_grant" {
			return nil, apperr.NewAuthExpiredError(
				"refresh token expired: run `schwab-agent auth login` to re-authenticate",
				nil,
			)
		}
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode(), string(resp.Bytes()))
	}

	// Parse new token data
	var newToken TokenData
	if err := json.Unmarshal(resp.Bytes(), &newToken); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Compute ExpiresAt: exchange time + expires_in
	newToken.ExpiresAt = float64(time.Now().Unix()) + float64(newToken.ExpiresIn)

	// Preserve original creation_timestamp
	return &TokenFile{
		CreationTimestamp: tf.CreationTimestamp,
		Token:             newToken,
	}, nil
}

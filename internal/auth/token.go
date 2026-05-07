package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	schwabauth "github.com/major/schwab-go/schwab/auth"

	"github.com/major/schwab-agent/internal/apperr"
)

// refreshTokenMaxAge is 6.5 days in seconds, matching schwab-go and schwab-py's default.
const refreshTokenMaxAge = 561600

// TokenFile is the on-disk token format shared with schwab-go.
type TokenFile = schwabauth.TokenFile

// TokenData holds the OAuth token fields shared with schwab-go.
type TokenData = schwabauth.TokenData

// LoadToken reads and parses the token file.
// Returns AuthRequiredError if the file doesn't exist.
func LoadToken(tokenPath string) (*TokenFile, error) {
	token, err := schwabauth.NewFileTokenStore(tokenPath).Load(context.Background())
	if err != nil {
		if schwabauth.IsRequired(err) {
			return nil, apperr.NewAuthRequiredError(
				"token file not found: run `schwab-agent auth login` to authenticate",
				err,
			)
		}
		return nil, fmt.Errorf("failed to load token file: %w", err)
	}

	return &token, nil
}

// SaveToken writes the token file with 0600 permissions.
// Creates the parent directory with 0700 if missing.
func SaveToken(tokenPath string, tf *TokenFile) error {
	if tf == nil {
		return errors.New("token file is required")
	}

	if err := schwabauth.NewFileTokenStore(tokenPath).Save(context.Background(), *tf); err != nil {
		return fmt.Errorf("failed to save token file: %w", err)
	}

	return nil
}

// IsAccessTokenExpired checks if the access token is expired (with 300s leeway).
// Returns true if [time.Now] is past ExpiresAt minus the leeway.
func IsAccessTokenExpired(tf *TokenFile) bool {
	if tf == nil {
		return true
	}

	return schwabauth.IsAccessTokenExpired(*tf)
}

// IsRefreshTokenStale checks if the refresh token is stale (> 6.5 days old).
// Returns true if [time.Now] is past creation_timestamp plus the max age.
func IsRefreshTokenStale(tf *TokenFile) bool {
	if tf == nil {
		return true
	}

	return time.Now().Unix() >= tf.CreationTimestamp+refreshTokenMaxAge
}

// RefreshAccessToken exchanges the refresh token for a new access token.
// Preserves creation_timestamp from the original TokenFile.
// Returns AuthExpiredError if the refresh fails with invalid_grant.
// The endpoint parameter allows overriding the token URL for testing.
func RefreshAccessToken(cfg *Config, tf *TokenFile, endpoint string) (*TokenFile, error) {
	if tf == nil {
		return nil, errors.New("token file is required")
	}

	schwabCfg, err := cfg.schwabAuthConfig(endpoint)
	if err != nil {
		return nil, err
	}

	refreshed, err := schwabauth.RefreshTokenFile(context.Background(), schwabCfg, *tf, cfg.oauthHTTPClient())
	if err != nil {
		if schwabauth.IsExpired(err) || strings.Contains(err.Error(), "invalid_grant") {
			return nil, apperr.NewAuthExpiredError(
				"refresh token expired: run `schwab-agent auth login` to re-authenticate",
				err,
			)
		}
		return nil, fmt.Errorf("refresh access token: %w", err)
	}

	return &refreshed, nil
}

func tokenEndpointOAuthBaseURL(endpoint string) string {
	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		return ""
	}

	return strings.TrimSuffix(strings.TrimRight(trimmedEndpoint, "/"), "/token")
}

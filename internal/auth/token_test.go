package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

// makeTokenFile creates a TokenFile with sensible defaults for testing.
func makeTokenFile(creationTimestamp int64, expiresAt int64) *TokenFile {
	return &TokenFile{
		CreationTimestamp: creationTimestamp,
		Token: TokenData{
			AccessToken:  "test-access-token",
			TokenType:    "Bearer",
			ExpiresIn:    1800,
			RefreshToken: "test-refresh-token",
			Scope:        "api",
			ExpiresAt:    expiresAt,
		},
	}
}

// --- LoadToken tests ---

func TestLoadToken_MissingFile_ReturnsAuthRequiredError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "nonexistent_token.json")

	// Act
	tf, err := LoadToken(tokenPath)

	// Assert
	assert.Nil(t, tf)
	require.Error(t, err)

	var authReqErr *apperr.AuthRequiredError
	require.ErrorAs(t, err, &authReqErr)
	assert.Contains(t, err.Error(), "token file not found")
}

func TestLoadToken_ValidFile_ReturnsTokenFile(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	now := time.Now()
	tf := makeTokenFile(now.Unix(), now.Add(30*time.Minute).Unix())

	data, err := json.Marshal(tf)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tokenPath, data, 0o600))

	// Act
	loaded, err := LoadToken(tokenPath)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, tf.CreationTimestamp, loaded.CreationTimestamp)
	assert.Equal(t, tf.Token.AccessToken, loaded.Token.AccessToken)
	assert.Equal(t, tf.Token.RefreshToken, loaded.Token.RefreshToken)
	assert.Equal(t, tf.Token.ExpiresIn, loaded.Token.ExpiresIn)
	assert.Equal(t, tf.Token.ExpiresAt, loaded.Token.ExpiresAt)
	assert.Equal(t, tf.Token.Scope, loaded.Token.Scope)
	assert.Equal(t, tf.Token.TokenType, loaded.Token.TokenType)
}

func TestLoadToken_InvalidJSON_ReturnsError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	require.NoError(t, os.WriteFile(tokenPath, []byte("not valid json"), 0o600))

	// Act
	tf, err := LoadToken(tokenPath)

	// Assert
	assert.Nil(t, tf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load token file")
}

func TestLoadToken_PermissionDenied_ReturnsError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	require.NoError(t, os.WriteFile(tokenPath, []byte("{}"), 0o000))

	// Act
	tf, err := LoadToken(tokenPath)

	// Assert
	assert.Nil(t, tf)
	require.Error(t, err)
	// Should NOT be AuthRequiredError (file exists but can't be read)
	_, ok := errors.AsType[*apperr.AuthRequiredError](err)
	assert.False(t, ok)
}

// --- SaveToken tests ---

func TestSaveToken_CreatesFileWithCorrectPermissions(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	now := time.Now()
	tf := makeTokenFile(now.Unix(), now.Add(30*time.Minute).Unix())

	// Act
	err := SaveToken(tokenPath, tf)

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, tokenPath)

	info, err := os.Stat(tokenPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestSaveToken_CreatesParentDirWithCorrectPermissions(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "subdir", "nested", "token.json")

	now := time.Now()
	tf := makeTokenFile(now.Unix(), now.Add(30*time.Minute).Unix())

	// Act
	err := SaveToken(tokenPath, tf)

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, tokenPath)

	parentDir := filepath.Dir(tokenPath)
	info, err := os.Stat(parentDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestSaveToken_RoundTrip_PreservesData(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	now := time.Now()
	original := makeTokenFile(now.Unix(), now.Add(30*time.Minute).Unix())

	// Act
	err := SaveToken(tokenPath, original)
	require.NoError(t, err)

	loaded, err := LoadToken(tokenPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, original.CreationTimestamp, loaded.CreationTimestamp)
	assert.Equal(t, original.Token.AccessToken, loaded.Token.AccessToken)
	assert.Equal(t, original.Token.RefreshToken, loaded.Token.RefreshToken)
	assert.Equal(t, original.Token.ExpiresIn, loaded.Token.ExpiresIn)
	assert.Equal(t, original.Token.ExpiresAt, loaded.Token.ExpiresAt)
	assert.Equal(t, original.Token.Scope, loaded.Token.Scope)
	assert.Equal(t, original.Token.TokenType, loaded.Token.TokenType)
}

func TestSaveToken_OverwritesExistingFile(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")

	now := time.Now()
	first := makeTokenFile(now.Unix(), now.Add(30*time.Minute).Unix())
	require.NoError(t, SaveToken(tokenPath, first))

	second := makeTokenFile(now.Unix(), now.Add(60*time.Minute).Unix())
	second.Token.AccessToken = "updated-access-token"

	// Act
	err := SaveToken(tokenPath, second)
	require.NoError(t, err)

	loaded, err := LoadToken(tokenPath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "updated-access-token", loaded.Token.AccessToken)
}

func TestSaveToken_MkdirAllFailure(t *testing.T) {
	// Arrange: use a path under /dev/null which cannot be a directory parent.
	badPath := "/dev/null/impossible/token.json"
	tf := makeTokenFile(1713700000, 1713701800)

	// Act
	err := SaveToken(badPath, tf)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save token file")
}

func TestSaveToken_WriteFileFailure(t *testing.T) {
	// Arrange: create a read-only directory so WriteFile fails.
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0o500))
	tokenPath := filepath.Join(readOnlyDir, "token.json")
	tf := makeTokenFile(1713700000, 1713701800)

	// Act
	err := SaveToken(tokenPath, tf)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save token file")
}

// --- IsAccessTokenExpired tests ---

func TestIsAccessTokenExpired_FutureExpiry_ReturnsFalse(t *testing.T) {
	// Token expires 30 minutes from now (well beyond 5-min leeway)
	tf := makeTokenFile(
		time.Now().Unix(),
		time.Now().Add(30*time.Minute).Unix(),
	)

	assert.False(t, IsAccessTokenExpired(tf))
}

func TestIsAccessTokenExpired_PastExpiry_ReturnsTrue(t *testing.T) {
	// Token expired 10 minutes ago
	tf := makeTokenFile(
		time.Now().Unix(),
		time.Now().Add(-10*time.Minute).Unix(),
	)

	assert.True(t, IsAccessTokenExpired(tf))
}

func TestIsAccessTokenExpired_WithinLeeway_ReturnsTrue(t *testing.T) {
	// Token expires in 4 minutes (within 5-min leeway)
	tf := makeTokenFile(
		time.Now().Unix(),
		time.Now().Add(4*time.Minute).Unix(),
	)

	assert.True(t, IsAccessTokenExpired(tf))
}

func TestIsAccessTokenExpired_ExactlyAtLeeway_ReturnsTrue(t *testing.T) {
	// Token expires in exactly 5 minutes (at the boundary, should be expired)
	tf := makeTokenFile(
		time.Now().Unix(),
		time.Now().Add(5*time.Minute).Unix(),
	)

	// ExpiresAt - 300 == now, so now >= ExpiresAt - 300 is true
	assert.True(t, IsAccessTokenExpired(tf))
}

func TestIsAccessTokenExpired_JustBeyondLeeway_ReturnsFalse(t *testing.T) {
	// Token expires in 5 minutes and 10 seconds (beyond the leeway)
	tf := makeTokenFile(
		time.Now().Unix(),
		time.Now().Add(5*time.Minute+10*time.Second).Unix(),
	)

	assert.False(t, IsAccessTokenExpired(tf))
}

// --- IsRefreshTokenStale tests ---

func TestIsRefreshTokenStale_RecentToken_ReturnsFalse(t *testing.T) {
	// Token created now
	tf := makeTokenFile(time.Now().Unix(), time.Now().Add(30*time.Minute).Unix())

	assert.False(t, IsRefreshTokenStale(tf))
}

func TestIsRefreshTokenStale_After6Days_ReturnsFalse(t *testing.T) {
	// Token created 6 days ago (< 6.5 days)
	sixDaysAgo := time.Now().Add(-6 * 24 * time.Hour).Unix()
	tf := makeTokenFile(sixDaysAgo, time.Now().Add(30*time.Minute).Unix())

	assert.False(t, IsRefreshTokenStale(tf))
}

func TestIsRefreshTokenStale_After7Days_ReturnsTrue(t *testing.T) {
	// Token created 7 days ago (> 6.5 days = 561600 seconds)
	sevenDaysAgo := time.Now().Add(-7 * 24 * time.Hour).Unix()
	tf := makeTokenFile(sevenDaysAgo, time.Now().Add(30*time.Minute).Unix())

	assert.True(t, IsRefreshTokenStale(tf))
}

func TestIsRefreshTokenStale_ExactlyAt561600Seconds_ReturnsTrue(t *testing.T) {
	// Token created exactly 561600 seconds ago (at the boundary)
	staleTime := time.Now().Unix() - 561600
	tf := makeTokenFile(staleTime, time.Now().Add(30*time.Minute).Unix())

	assert.True(t, IsRefreshTokenStale(tf))
}

func TestIsRefreshTokenStale_JustBefore561600Seconds_ReturnsFalse(t *testing.T) {
	// Token created 561599 seconds ago (just under the threshold)
	notStaleTime := time.Now().Unix() - 561599
	tf := makeTokenFile(notStaleTime, time.Now().Add(30*time.Minute).Unix())

	assert.False(t, IsRefreshTokenStale(tf))
}

// --- RefreshAccessToken tests ---

func TestRefreshAccessToken_Success_ReturnsNewTokenFile(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		// Verify request method
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify Content-Type
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		// Verify Basic Auth
		authHeader := r.Header.Get("Authorization")
		if !assert.True(t, strings.HasPrefix(authHeader, "Basic ")) {
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, "test-client:test-secret", string(decoded))

		// Verify body
		if !assert.NoError(t, r.ParseForm()) {
			return
		}
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "test-refresh-token", r.Form.Get("refresh_token"))

		// Return new token
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access-token",
			"token_type":    "Bearer",
			"expires_in":    1800,
			"refresh_token": "new-refresh-token",
			"scope":         "api",
		}))
	}))
	defer server.Close()

	cfg := &Config{ClientID: "test-client", ClientSecret: "test-secret", BaseURLInsecure: true}

	originalCreation := time.Now().Add(-1 * time.Hour).Unix()
	tf := makeTokenFile(originalCreation, time.Now().Add(-5*time.Minute).Unix())

	// Act
	newTF, err := RefreshAccessToken(cfg, tf, server.URL)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, newTF)

	assert.Equal(t, "new-access-token", newTF.Token.AccessToken)
	assert.Equal(t, "new-refresh-token", newTF.Token.RefreshToken)
	assert.Equal(t, "Bearer", newTF.Token.TokenType)
	assert.Equal(t, 1800, newTF.Token.ExpiresIn)
	assert.Equal(t, "api", newTF.Token.Scope)

	// creation_timestamp MUST be preserved
	assert.Equal(t, originalCreation, newTF.CreationTimestamp)

	// ExpiresAt should be computed (approximately now + ExpiresIn)
	assert.InDelta(t, time.Now().Unix()+1800, newTF.Token.ExpiresAt, 5.0)
}

func TestRefreshAccessToken_UsesDerivedTokenURLAndInsecureTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/proxy/v1/oauth/token", r.URL.Path)
		if !assert.NoError(t, r.ParseForm()) {
			return
		}
		assert.Equal(t, "refresh_token", r.Form.Get("grant_type"))
		assert.Equal(t, "test-refresh-token", r.Form.Get("refresh_token"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "derived-access-token",
			"token_type":    "Bearer",
			"expires_in":    1800,
			"refresh_token": "derived-refresh-token",
			"scope":         "api",
		}))
	}))
	defer server.Close()

	cfg := &Config{
		ClientID:        "test-client",
		ClientSecret:    "test-secret",
		BaseURL:         server.URL + "/proxy/",
		BaseURLInsecure: true,
	}

	tf := makeTokenFile(time.Now().Add(-1*time.Hour).Unix(), time.Now().Add(-5*time.Minute).Unix())

	newTF, err := RefreshAccessToken(cfg, tf, "")
	require.NoError(t, err)
	require.NotNil(t, newTF)
	assert.Equal(t, "derived-access-token", newTF.Token.AccessToken)
	assert.Equal(t, "derived-refresh-token", newTF.Token.RefreshToken)
}

func TestRefreshAccessToken_PreservesCreationTimestamp(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "refreshed-token",
			"token_type":    "Bearer",
			"expires_in":    1800,
			"refresh_token": "refreshed-refresh",
			"scope":         "api",
		}))
	}))
	defer server.Close()

	cfg := &Config{ClientID: "id", ClientSecret: "secret", BaseURLInsecure: true}

	// Creation was 3 days ago
	threeDaysAgo := time.Now().Add(-3 * 24 * time.Hour).Unix()
	tf := makeTokenFile(threeDaysAgo, time.Now().Add(-1*time.Minute).Unix())

	// Act
	newTF, err := RefreshAccessToken(cfg, tf, server.URL)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, threeDaysAgo, newTF.CreationTimestamp, "creation_timestamp must be preserved across refresh")
}

func TestRefreshAccessToken_InvalidGrant_ReturnsAuthExpiredError(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid_grant",
		}))
	}))
	defer server.Close()

	cfg := &Config{ClientID: "id", ClientSecret: "secret", BaseURLInsecure: true}
	tf := makeTokenFile(time.Now().Unix(), time.Now().Add(30*time.Minute).Unix())

	// Act
	newTF, err := RefreshAccessToken(cfg, tf, server.URL)

	// Assert
	assert.Nil(t, newTF)
	require.Error(t, err)

	var authExpErr *apperr.AuthExpiredError
	require.ErrorAs(t, err, &authExpErr)
	assert.Contains(t, err.Error(), "refresh token expired")
}

func TestRefreshAccessToken_ServerError_ReturnsError(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error")
	}))
	defer server.Close()

	cfg := &Config{ClientID: "id", ClientSecret: "secret", BaseURLInsecure: true}
	tf := makeTokenFile(time.Now().Unix(), time.Now().Add(30*time.Minute).Unix())

	// Act
	newTF, err := RefreshAccessToken(cfg, tf, server.URL)

	// Assert
	assert.Nil(t, newTF)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refresh access token")
}

func TestRefreshAccessToken_InvalidResponseJSON_ReturnsError(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "not valid json")
	}))
	defer server.Close()

	cfg := &Config{ClientID: "id", ClientSecret: "secret", BaseURLInsecure: true}
	tf := makeTokenFile(time.Now().Unix(), time.Now().Add(30*time.Minute).Unix())

	// Act
	newTF, err := RefreshAccessToken(cfg, tf, server.URL)

	// Assert
	assert.Nil(t, newTF)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse token refresh response")
}

func TestRefreshAccessToken_NetworkError_ReturnsError(t *testing.T) {
	// Arrange: use a URL that will fail to connect
	cfg := &Config{ClientID: "id", ClientSecret: "secret"}
	tf := makeTokenFile(time.Now().Unix(), time.Now().Add(30*time.Minute).Unix())

	// Act
	newTF, err := RefreshAccessToken(cfg, tf, "http://127.0.0.1:0/nonexistent")

	// Assert
	assert.Nil(t, newTF)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oauth_base_url")
}

func TestRefreshAccessToken_OtherHTTPError_ReturnsGenericError(t *testing.T) {
	// Arrange: non-invalid_grant 400 response
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid_client",
		}))
	}))
	defer server.Close()

	cfg := &Config{ClientID: "id", ClientSecret: "secret", BaseURLInsecure: true}
	tf := makeTokenFile(time.Now().Unix(), time.Now().Add(30*time.Minute).Unix())

	// Act
	newTF, err := RefreshAccessToken(cfg, tf, server.URL)

	// Assert
	assert.Nil(t, newTF)
	require.Error(t, err)

	// Should NOT be AuthExpiredError for non-invalid_grant errors
	_, ok := errors.AsType[*apperr.AuthExpiredError](err)
	assert.False(t, ok)
}

// --- Token JSON format compatibility ---

func TestTokenFile_JSONFormat_MatchesSchwabGo(t *testing.T) {
	// Verify the JSON format matches schwab-go's expected structure:
	// {"creation_timestamp": <int>, "token": {"access_token": ..., ...}}
	tf := makeTokenFile(1713700000, 1713701800)

	data, err := json.Marshal(tf)
	require.NoError(t, err)

	// Parse as generic map to verify structure
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	// Top-level keys must be creation_timestamp and token
	assert.Contains(t, raw, "creation_timestamp")
	assert.Contains(t, raw, "token")
	assert.Len(t, raw, 2)

	// Token must be a nested object with expected keys
	tokenMap, ok := raw["token"].(map[string]any)
	require.True(t, ok, "token field must be an object")
	assert.Contains(t, tokenMap, "access_token")
	assert.Contains(t, tokenMap, "token_type")
	assert.Contains(t, tokenMap, "expires_in")
	assert.Contains(t, tokenMap, "refresh_token")
	assert.Contains(t, tokenMap, "scope")
	assert.Contains(t, tokenMap, "expires_at")
}

func TestTokenFile_JSONRoundTrip_FromSchwabGoFormat(t *testing.T) {
	// Simulate loading a token file written by schwab-go.
	schwabPyJSON := `{
		"creation_timestamp": 1713700000,
		"token": {
			"access_token": "py-access-token",
			"token_type": "Bearer",
			"expires_in": 1800,
			"refresh_token": "py-refresh-token",
			"scope": "api",
			"expires_at": 1713701800
		}
	}`

	var tf TokenFile
	err := json.Unmarshal([]byte(schwabPyJSON), &tf)
	require.NoError(t, err)

	assert.Equal(t, int64(1713700000), tf.CreationTimestamp)
	assert.Equal(t, "py-access-token", tf.Token.AccessToken)
	assert.Equal(t, "Bearer", tf.Token.TokenType)
	assert.Equal(t, 1800, tf.Token.ExpiresIn)
	assert.Equal(t, "py-refresh-token", tf.Token.RefreshToken)
	assert.Equal(t, "api", tf.Token.Scope)
	assert.Equal(t, int64(1713701800), tf.Token.ExpiresAt)
}

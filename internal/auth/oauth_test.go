package auth

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

// synchronizedBuffer is a concurrency-safe writer for login flow tests.
type synchronizedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write appends data to the buffer safely.
func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

// String returns the current buffer contents.
func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func TestAuthorizeURL_ReturnsExpectedParametersAndState(t *testing.T) {
	// Arrange
	cfg := &Config{
		ClientID:     "test-client",
		CallbackURL:  "https://127.0.0.1:8182",
		ClientSecret: "unused",
	}

	// Act
	authURL, state, err := AuthorizeURL(cfg)

	// Assert
	require.NoError(t, err)

	parsedURL, err := url.Parse(authURL)
	require.NoError(t, err)

	assert.Equal(t, cfg.OAuthAuthorizeURL(), parsedURL.Scheme+"://"+parsedURL.Host+parsedURL.Path)
	assert.Equal(t, state, parsedURL.Query().Get("state"))
	assert.Equal(t, "test-client", parsedURL.Query().Get("client_id"))
	assert.Equal(t, "https://127.0.0.1:8182", parsedURL.Query().Get("redirect_uri"))
	assert.Equal(t, "code", parsedURL.Query().Get("response_type"))
	assert.Equal(t, "api", parsedURL.Query().Get("scope"))
	assert.Len(t, state, 64)
	_, err = hex.DecodeString(state)
	require.NoError(t, err)
}

func TestAuthorizeURL_UsesDerivedBaseURL(t *testing.T) {
	cfg := &Config{
		ClientID:     "test-client",
		ClientSecret: "unused",
		CallbackURL:  "https://127.0.0.1:8182",
		BaseURL:      "https://proxy.example.com/prefix",
	}

	authURL, _, err := AuthorizeURL(cfg)
	require.NoError(t, err)

	parsedURL, err := url.Parse(authURL)
	require.NoError(t, err)
	assert.Equal(t, "https://proxy.example.com/prefix/v1/oauth/authorize", parsedURL.Scheme+"://"+parsedURL.Host+parsedURL.Path)
}

func TestExchangeCode_Success_UsesBasicAuthAndReturnsTokenFile(t *testing.T) {
	// Arrange
	fixedNow := time.Date(2026, time.April, 21, 12, 0, 0, 0, time.UTC)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		authHeader := r.Header.Get("Authorization")
		require.True(t, strings.HasPrefix(authHeader, "Basic "))
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
		require.NoError(t, err)
		assert.Equal(t, "client-id:client-secret", string(decoded))

		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "auth-code-123", r.Form.Get("code"))
		assert.Equal(t, "https://127.0.0.1:8182", r.Form.Get("redirect_uri"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access-token",
			"token_type":    "Bearer",
			"expires_in":    1800,
			"refresh_token": "new-refresh-token",
			"scope":         "api",
		}))
	}))
	defer server.Close()

	cfg := &Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		CallbackURL:  "https://127.0.0.1:8182",
	}

	// Act
	tokenFile, err := ExchangeCode(cfg, "auth-code-123", server.URL, fixedNow)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, tokenFile)
	assert.Equal(t, fixedNow.Unix(), tokenFile.CreationTimestamp)
	assert.Equal(t, "new-access-token", tokenFile.Token.AccessToken)
	assert.Equal(t, "new-refresh-token", tokenFile.Token.RefreshToken)
	assert.Equal(t, "Bearer", tokenFile.Token.TokenType)
	assert.Equal(t, 1800, tokenFile.Token.ExpiresIn)
	assert.Equal(t, "api", tokenFile.Token.Scope)
	assert.Equal(t, float64(fixedNow.Unix()+1800), tokenFile.Token.ExpiresAt)
}

func TestExchangeCode_Failure_ReturnsAuthCallbackError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_request"}`))
	}))
	defer server.Close()

	cfg := &Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		CallbackURL:  "https://127.0.0.1:8182",
	}

	// Act
	tokenFile, err := ExchangeCode(cfg, "bad-code", server.URL, time.Now())

	// Assert
	assert.Nil(t, tokenFile)
	require.Error(t, err)
	var callbackErr *apperr.AuthCallbackError
	assert.ErrorAs(t, err, &callbackErr)
	assert.Contains(t, err.Error(), "token exchange failed")
}

func TestExchangeCode_UsesDerivedTokenURLAndInsecureTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/proxy/v1/oauth/token", r.URL.Path)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "derived-code", r.Form.Get("code"))
		assert.Equal(t, "https://127.0.0.1:8182", r.Form.Get("redirect_uri"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "proxy-access-token",
			"token_type":    "Bearer",
			"expires_in":    1800,
			"refresh_token": "proxy-refresh-token",
			"scope":         "api",
		}))
	}))
	defer server.Close()

	cfg := &Config{
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
		CallbackURL:     "https://127.0.0.1:8182",
		BaseURL:         server.URL + "/proxy/",
		BaseURLInsecure: true,
	}

	tokenFile, err := ExchangeCode(cfg, "derived-code", "", time.Unix(1_700_000_000, 0))
	require.NoError(t, err)
	require.NotNil(t, tokenFile)
	assert.Equal(t, "proxy-access-token", tokenFile.Token.AccessToken)
	assert.Equal(t, "proxy-refresh-token", tokenFile.Token.RefreshToken)
}

func TestStartCallbackServer_Success_ReturnsCode(t *testing.T) {
	// Arrange
	addr := freeLoopbackAddress(t)
	resultCh := make(chan callbackResult, 1)

	go func() {
		code, err := StartCallbackServer(addr, "expected-state")
		resultCh <- callbackResult{code: code, err: err}
	}()

	// Act
	responseBody, statusCode := sendCallbackRequest(t, addr, "valid-code", "expected-state")
	result := <-resultCh

	// Assert
	require.NoError(t, result.err)
	assert.Equal(t, "valid-code", result.code)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, responseBody, "Authentication successful! You can close this tab.")
}

func TestStartCallbackServer_StateMismatch_ReturnsAuthCallbackError(t *testing.T) {
	// Arrange
	addr := freeLoopbackAddress(t)
	resultCh := make(chan callbackResult, 1)

	go func() {
		code, err := StartCallbackServer(addr, "expected-state")
		resultCh <- callbackResult{code: code, err: err}
	}()

	// Act
	responseBody, statusCode := sendCallbackRequest(t, addr, "valid-code", "wrong-state")
	result := <-resultCh

	// Assert
	assert.Empty(t, result.code)
	require.Error(t, result.err)
	var callbackErr *apperr.AuthCallbackError
	assert.ErrorAs(t, result.err, &callbackErr)
	assert.Equal(t, http.StatusBadRequest, statusCode)
	assert.Contains(t, responseBody, "state mismatch")
}

func TestRunLogin_PrintsURLAndSavesToken(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")
	callbackAddr := freeLoopbackAddress(t)
	cfg := &Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		CallbackURL:  "https://" + callbackAddr,
	}

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "login-code", r.Form.Get("code"))
		assert.Equal(t, cfg.CallbackURL, r.Form.Get("redirect_uri"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "saved-access-token",
			"token_type":    "Bearer",
			"expires_in":    900,
			"refresh_token": "saved-refresh-token",
			"scope":         "api",
		}))
	}))
	defer tokenServer.Close()

	writer := &synchronizedBuffer{}
	errCh := make(chan error, 1)
	before := time.Now().Unix()

	go func() {
		errCh <- RunLogin(cfg, tokenPath, tokenServer.URL, false, writer)
	}()

	authURL := waitForAuthURL(t, writer)
	parsedURL, err := url.Parse(strings.TrimSpace(authURL))
	require.NoError(t, err)

	// Act
	_, statusCode := sendCallbackRequest(t, callbackAddr, "login-code", parsedURL.Query().Get("state"))
	err = <-errCh
	after := time.Now().Unix()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, writer.String(), cfg.OAuthAuthorizeURL())

	tokenFile, err := LoadToken(tokenPath)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, tokenFile.CreationTimestamp, before)
	assert.LessOrEqual(t, tokenFile.CreationTimestamp, after)
	assert.Equal(t, "saved-access-token", tokenFile.Token.AccessToken)
	assert.Equal(t, "saved-refresh-token", tokenFile.Token.RefreshToken)
	assert.InDelta(t, float64(tokenFile.CreationTimestamp+900), tokenFile.Token.ExpiresAt, 1)
}

func TestValidateCallbackAddr_EdgeCases(t *testing.T) {
	t.Run("empty addr returns default", func(t *testing.T) {
		addr, err := validateCallbackAddr("")
		require.NoError(t, err)
		assert.Equal(t, defaultCallbackAddr, addr)
	})

	t.Run("missing port returns error", func(t *testing.T) {
		_, err := validateCallbackAddr("127.0.0.1")
		require.Error(t, err)
		var callbackErr *apperr.AuthCallbackError
		assert.ErrorAs(t, err, &callbackErr)
		assert.Contains(t, err.Error(), "host and port")
	})

	t.Run("non-loopback host returns error", func(t *testing.T) {
		_, err := validateCallbackAddr("192.168.1.1:8182")
		require.Error(t, err)
		var callbackErr *apperr.AuthCallbackError
		assert.ErrorAs(t, err, &callbackErr)
		assert.Contains(t, err.Error(), "127.0.0.1")
	})

	t.Run("valid loopback addr passes through", func(t *testing.T) {
		addr, err := validateCallbackAddr("127.0.0.1:9999")
		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1:9999", addr)
	})
}

// freeLoopbackAddress reserves and returns an available loopback port for tests.
func freeLoopbackAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	return listener.Addr().String()
}

// sendCallbackRequest sends a callback request to the local HTTPS server and returns the response.
func sendCallbackRequest(t *testing.T, addr, code, state string) (respBody string, respStatus int) {
	t.Helper()

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	callbackURL := "https://" + addr + "/?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state)

	var response *http.Response
	require.Eventually(t, func() bool {
		resp, err := client.Get(callbackURL)
		if err != nil {
			return false
		}
		response = resp
		return true
	}, 5*time.Second, 25*time.Millisecond)
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return string(body), response.StatusCode
}

// waitForAuthURL waits until RunLogin writes the authorization URL.
func waitForAuthURL(t *testing.T, writer *synchronizedBuffer) string {
	t.Helper()

	var authURL string
	require.Eventually(t, func() bool {
		authURL = strings.TrimSpace(writer.String())
		return authURL != ""
	}, 5*time.Second, 25*time.Millisecond)

	return authURL
}

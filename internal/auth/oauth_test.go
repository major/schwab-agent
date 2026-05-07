package auth

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
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

	schwabauth "github.com/major/schwab-go/schwab/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

// synchronizedBuffer is a concurrency-safe writer for login flow tests.
type synchronizedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

type callbackResult struct {
	code string
	err  error
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
	assert.Empty(t, parsedURL.Query().Get("scope"))
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
	assert.Equal(
		t,
		"https://proxy.example.com/prefix/v1/oauth/authorize",
		parsedURL.Scheme+"://"+parsedURL.Host+parsedURL.Path,
	)
}

func TestAuthorizeURL_InvalidConfigReturnsError(t *testing.T) {
	cfg := &Config{
		ClientID:     "test-client",
		ClientSecret: "unused",
		BaseURL:      "http://proxy.example.com",
	}

	authURL, state, err := AuthorizeURL(cfg)

	assert.Empty(t, authURL)
	assert.Empty(t, state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to build OAuth authorization URL")
}

func TestExchangeCode_Success_UsesBasicAuthAndReturnsTokenFile(t *testing.T) {
	// Arrange
	fixedNow := time.Date(2026, time.April, 21, 12, 0, 0, 0, time.UTC)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		authHeader := r.Header.Get("Authorization")
		if !assert.True(t, strings.HasPrefix(authHeader, "Basic ")) {
			return
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, "client-id:client-secret", string(decoded))

		if !assert.NoError(t, r.ParseForm()) {
			return
		}
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "auth-code-123", r.Form.Get("code"))
		assert.Equal(t, "https://127.0.0.1:8182", r.Form.Get("redirect_uri"))

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

	cfg := &Config{
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
		CallbackURL:     "https://127.0.0.1:8182",
		BaseURLInsecure: true,
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
	assert.Equal(t, fixedNow.Unix()+1800, tokenFile.Token.ExpiresAt)
}

func TestExchangeCode_Failure_ReturnsAuthCallbackError(t *testing.T) {
	// Arrange
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_request"}`))
	}))
	defer server.Close()

	cfg := &Config{
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
		CallbackURL:     "https://127.0.0.1:8182",
		BaseURLInsecure: true,
	}

	// Act
	tokenFile, err := ExchangeCode(cfg, "bad-code", server.URL, time.Now())

	// Assert
	assert.Nil(t, tokenFile)
	require.Error(t, err)
	var callbackErr *apperr.AuthCallbackError
	require.ErrorAs(t, err, &callbackErr)
	assert.Contains(t, err.Error(), "token exchange failed")
}

func TestExchangeCode_InvalidConfigReturnsError(t *testing.T) {
	tokenFile, err := ExchangeCode(nil, "auth-code", "", time.Now())

	assert.Nil(t, tokenFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to prepare token exchange")
}

func TestExchangeCode_UsesDerivedTokenURLAndInsecureTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/proxy/v1/oauth/token", r.URL.Path)
		if !assert.NoError(t, r.ParseForm()) {
			return
		}
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "derived-code", r.Form.Get("code"))
		assert.Equal(t, "https://127.0.0.1:8182", r.Form.Get("redirect_uri"))

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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
	assert.Contains(t, responseBody, "Login successful. You can close this tab.")
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
	require.ErrorAs(t, result.err, &callbackErr)
	// schwab-go writes the browser response before schwab-agent validates the
	// returned state. The CLI still receives AuthCallbackError, but the browser
	// sees schwab-go's generic success page until upstream exposes response
	// control for state validation failures.
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Contains(t, responseBody, "Login successful. You can close this tab.")
}

func TestRunLogin_PrintsURLAndSavesToken(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")
	callbackAddr := freeLoopbackAddress(t)
	cfg := &Config{
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
		CallbackURL:     "https://" + callbackAddr,
		BaseURLInsecure: true,
	}

	tokenServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		if !assert.NoError(t, r.ParseForm()) {
			return
		}
		assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
		assert.Equal(t, "login-code", r.Form.Get("code"))
		assert.Equal(t, cfg.CallbackURL, r.Form.Get("redirect_uri"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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
	assert.Contains(t, writer.String(), "/authorize")

	tokenFile, err := LoadToken(tokenPath)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, tokenFile.CreationTimestamp, before)
	assert.LessOrEqual(t, tokenFile.CreationTimestamp, after)
	assert.Equal(t, "saved-access-token", tokenFile.Token.AccessToken)
	assert.Equal(t, "saved-refresh-token", tokenFile.Token.RefreshToken)
	assert.InDelta(t, tokenFile.CreationTimestamp+900, tokenFile.Token.ExpiresAt, 1)
}

func TestStartCallbackServer_MissingCode_ReturnsAuthCallbackError(t *testing.T) {
	// Arrange
	addr := freeLoopbackAddress(t)
	resultCh := make(chan callbackResult, 1)

	go func() {
		code, err := StartCallbackServer(addr, "expected-state")
		resultCh <- callbackResult{code: code, err: err}
	}()

	// Act - send request with valid state but empty code
	responseBody, statusCode := sendCallbackRequest(t, addr, "", "expected-state")
	result := <-resultCh

	// Assert
	assert.Empty(t, result.code)
	require.Error(t, result.err)
	var callbackErr *apperr.AuthCallbackError
	require.ErrorAs(t, result.err, &callbackErr)
	// The exact delegated schwab-go callback error text may change; this assertion
	// verifies schwab-agent still maps it into the stable app callback category.
	assert.Contains(t, result.err.Error(), "OAuth callback failed")
	assert.Equal(t, http.StatusBadRequest, statusCode)
	assert.Contains(t, responseBody, "missing code")
}

func TestStartCallbackServer_InvalidAddressReturnsAuthCallbackError(t *testing.T) {
	code, err := StartCallbackServer("https://localhost:8182", "expected-state")

	assert.Empty(t, code)
	require.Error(t, err)
	var callbackErr *apperr.AuthCallbackError
	require.ErrorAs(t, err, &callbackErr)
	assert.Contains(t, err.Error(), "invalid OAuth callback URL")
	assert.Contains(t, errors.Unwrap(err).Error(), "127.0.0.1")
}

func TestStartCallbackServer_NonHTTPSAddressReturnsAuthCallbackError(t *testing.T) {
	code, err := StartCallbackServer("http://127.0.0.1:8182", "expected-state")

	assert.Empty(t, code)
	require.Error(t, err)
	var callbackErr *apperr.AuthCallbackError
	require.ErrorAs(t, err, &callbackErr)
	assert.Contains(t, err.Error(), "invalid OAuth callback URL")
	assert.Contains(t, errors.Unwrap(err).Error(), "must use https")
}

func TestRunLogin_InvalidConfigReturnsError(t *testing.T) {
	err := RunLogin(nil, filepath.Join(t.TempDir(), "token.json"), "", false, io.Discard)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to prepare OAuth login")
}

func TestRunLogin_WriteAuthorizationURLFailureReturnsError(t *testing.T) {
	callbackAddr := freeLoopbackAddress(t)
	cfg := &Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		CallbackURL:  "https://" + callbackAddr,
	}

	err := RunLogin(cfg, filepath.Join(t.TempDir(), "token.json"), "", false, errWriter{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write authorization URL")
}

func TestRunLogin_TokenExchangeFailureReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "token.json")
	callbackAddr := freeLoopbackAddress(t)
	cfg := &Config{
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
		CallbackURL:     "https://" + callbackAddr,
		BaseURLInsecure: true,
	}

	tokenServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/token", r.URL.Path)
		w.WriteHeader(http.StatusBadRequest)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request"}))
	}))
	defer tokenServer.Close()

	writer := &synchronizedBuffer{}
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunLogin(cfg, tokenPath, tokenServer.URL, false, writer)
	}()

	authURL := waitForAuthURL(t, writer)
	parsedURL, err := url.Parse(strings.TrimSpace(authURL))
	require.NoError(t, err)
	_, statusCode := sendCallbackRequest(t, callbackAddr, "login-code", parsedURL.Query().Get("state"))
	err = <-errCh

	assert.Equal(t, http.StatusOK, statusCode)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OAuth login failed")
}

func TestMapSchwabAuthError_ReturnsExpectedAppErrors(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		check     func(*testing.T, error)
		wantError bool
	}{
		{
			name:      "nil",
			err:       nil,
			wantError: false,
		},
		{
			name: "callback",
			err:  &schwabauth.AuthCallbackError{Msg: "callback failed"},
			check: func(t *testing.T, err error) {
				t.Helper()
				var callbackErr *apperr.AuthCallbackError
				require.ErrorAs(t, err, &callbackErr)
			},
			wantError: true,
		},
		{
			name: "expired",
			err:  &schwabauth.AuthExpiredError{Msg: "expired"},
			check: func(t *testing.T, err error) {
				t.Helper()
				var expiredErr *apperr.AuthExpiredError
				require.ErrorAs(t, err, &expiredErr)
			},
			wantError: true,
		},
		{
			name: "required",
			err:  &schwabauth.AuthRequiredError{Msg: "required"},
			check: func(t *testing.T, err error) {
				t.Helper()
				var requiredErr *apperr.AuthRequiredError
				require.ErrorAs(t, err, &requiredErr)
			},
			wantError: true,
		},
		{
			name:      "generic",
			err:       errors.New("plain error"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapSchwabAuthError("wrapped", tt.err)

			if !tt.wantError {
				assert.NoError(t, got)
				return
			}

			require.Error(t, got)
			assert.Contains(t, got.Error(), "wrapped")
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, errors.New("writer failed")
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
func sendCallbackRequest(t *testing.T, addr, code, state string) (string, int) {
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

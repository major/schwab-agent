// Package client provides an authenticated HTTP client for the Schwab API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/major/schwab-agent/internal/apperr"
)

// testResponse is a simple struct used for JSON round-trip tests.
type testResponse struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// testRequestBody is a simple struct used for request body marshaling tests.
type testRequestBody struct {
	Symbol   string  `json:"symbol"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient("test-token")

	assert.Equal(t, "https://api.schwabapi.com", c.baseURL)
	assert.Equal(t, "test-token", c.token)
	assert.NotNil(t, c.httpClient)
	assert.NotNil(t, c.logger)
	assert.Equal(t, 30*time.Second, c.httpClient.Timeout, "default client must have a timeout to prevent hanging requests")
}

func TestNewClient_WithBaseURL(t *testing.T) {
	c := NewClient("tok", WithBaseURL("https://custom.api.com"))

	assert.Equal(t, "https://custom.api.com", c.baseURL)
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	custom := &http.Client{Timeout: 42 * time.Second}
	c := NewClient("tok", WithHTTPClient(custom))

	assert.Equal(t, custom, c.httpClient)
	assert.Equal(t, 42*time.Second, c.httpClient.Timeout, "WithHTTPClient must override the default timeout")
}

func TestNewClient_WithLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := NewClient("tok", WithLogger(logger))

	assert.Equal(t, logger, c.logger)
}

func TestDoGet_BearerTokenInHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-secret-token", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(testResponse{Name: "ok", Value: 1}))
	}))
	defer srv.Close()

	c := NewClient("my-secret-token", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/test", nil, &result)

	require.NoError(t, err)
	assert.Equal(t, "ok", result.Name)
	assert.Equal(t, 1, result.Value)
}

func TestDoGet_WithQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "AAPL", r.URL.Query().Get("symbol"))
		assert.Equal(t, "quote", r.URL.Query().Get("fields"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(testResponse{Name: "AAPL", Value: 150}))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/quotes", map[string]string{
		"symbol": "AAPL",
		"fields": "quote",
	}, &result)

	require.NoError(t, err)
	assert.Equal(t, "AAPL", result.Name)
	assert.Equal(t, 150, result.Value)
}

func TestDoGet_SpecialCharsInQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify that special characters are properly percent-encoded and
		// decoded by the server. Prior to the url.Values fix, raw string
		// concatenation would pass these through unencoded, breaking the request.
		assert.Equal(t, "BRK B", r.URL.Query().Get("symbol"))
		assert.Equal(t, "quote,fundamental", r.URL.Query().Get("fields"))
		assert.Equal(t, "a&b=c", r.URL.Query().Get("tricky"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(testResponse{Name: "BRK B", Value: 300}))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/quotes", map[string]string{
		"symbol": "BRK B",
		"fields": "quote,fundamental",
		"tricky": "a&b=c",
	}, &result)

	require.NoError(t, err)
	assert.Equal(t, "BRK B", result.Name)
}

func TestDoGet_NilResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	err := c.doGet(context.Background(), "/ping", nil, nil)

	require.NoError(t, err)
}

func TestDoPost_JSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPost, r.Method)

		var body testRequestBody
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "AAPL", body.Symbol)
		assert.Equal(t, 10, body.Quantity)
		assert.Equal(t, 150.50, body.Price)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(testResponse{Name: "order-123", Value: 1}))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doPost(context.Background(), "/orders", testRequestBody{
		Symbol:   "AAPL",
		Quantity: 10,
		Price:    150.50,
	}, &result)

	require.NoError(t, err)
	assert.Equal(t, "order-123", result.Name)
}

func TestDoPost_NilBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		assert.Empty(t, body)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	err := c.doPost(context.Background(), "/action", nil, nil)

	require.NoError(t, err)
}

func TestDoPut_JSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPut, r.Method)

		var body testRequestBody
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "MSFT", body.Symbol)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	err := c.doPut(context.Background(), "/orders/123", testRequestBody{
		Symbol:   "MSFT",
		Quantity: 5,
		Price:    300.00,
	}, nil)

	require.NoError(t, err)
}

func TestDoDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "Bearer del-token", r.Header.Get("Authorization"))
		assert.Equal(t, "/orders/456", r.URL.Path)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("del-token", WithBaseURL(srv.URL))
	err := c.doDelete(context.Background(), "/orders/456", nil)

	require.NoError(t, err)
}

func TestDoRequest_401_ReturnsAuthExpiredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"token expired"}`))
	}))
	defer srv.Close()

	c := NewClient("bad-token", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/accounts", nil, &result)

	require.Error(t, err)

	var authErr *apperr.AuthExpiredError
	require.ErrorAs(t, err, &authErr)
	assert.Contains(t, authErr.Error(), "authentication expired")
}

func TestDoRequest_404_ReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/accounts/999", nil, &result)

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, 404, httpErr.StatusCode)
	assert.Contains(t, httpErr.Body, "not found")
}

func TestDoRequest_500_ReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/broken", nil, &result)

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, 500, httpErr.StatusCode)
	assert.Contains(t, httpErr.Body, "internal server error")
}

func TestDoRequest_403_ReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/admin", nil, &result)

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, 403, httpErr.StatusCode)
}

func TestDoRequest_429_ReturnsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/quotes", nil, &result)

	require.Error(t, err)

	var httpErr *apperr.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, 429, httpErr.StatusCode)
	assert.Contains(t, httpErr.Body, "rate limited")
}

func TestDoGet_ContentTypeHeaders(t *testing.T) {
	// GET requests must NOT include Content-Type. The Schwab API returns 400
	// on GET requests that include Content-Type: application/json.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Empty(t, r.Header.Get("Content-Type"), "GET requests must not set Content-Type")

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(testResponse{Name: "ok", Value: 1}))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/test", nil, &result)

	require.NoError(t, err)
}

func TestDoRequest_PathConcatenation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/accounts/123/orders", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(testResponse{Name: "order", Value: 1}))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/v1/accounts/123/orders", nil, &result)

	require.NoError(t, err)
}

func TestDoRequest_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var result testResponse
	err := c.doGet(ctx, "/test", nil, &result)

	require.Error(t, err)
}

func TestDoRequest_EmptyResponseBody_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	err := c.doDelete(context.Background(), "/orders/789", nil)

	require.NoError(t, err)
}

func TestDoRequest_TokenUpdatedDirectly(t *testing.T) {
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(testResponse{Name: "ok", Value: 1}))
	}))
	defer srv.Close()

	c := NewClient("first-token", WithBaseURL(srv.URL))

	// First request with original token.
	var result testResponse
	err := c.doGet(context.Background(), "/test", nil, &result)
	require.NoError(t, err)
	assert.Equal(t, "Bearer first-token", capturedAuth)

	// Update token directly and verify next request uses it.
	c.token = "refreshed-token"
	err = c.doGet(context.Background(), "/test", nil, &result)
	require.NoError(t, err)
	assert.Equal(t, "Bearer refreshed-token", capturedAuth)
}

func TestDoRequest_JSONDecodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/test", nil, &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestDoRequest_ResponseBodyCappedAtMaxSize(t *testing.T) {
	// Verify that responses larger than maxResponseSize are silently truncated
	// rather than consuming unbounded memory. The truncated JSON will fail to
	// decode, which is the expected (safe) behavior.
	oversizedBody := bytes.Repeat([]byte("x"), maxResponseSize+1024)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(oversizedBody)
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/huge", nil, &result)

	// The response is truncated, so JSON decode fails. The important thing
	// is that we don't OOM - we get a clean decode error instead.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestDoRequest_UserAgentHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "schwab-agent/dev", r.Header.Get("User-Agent"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(testResponse{Name: "ok", Value: 1}))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/test", nil, &result)

	require.NoError(t, err)
}

func TestDoRequest_CustomUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "schwab-agent/1.2.3", r.Header.Get("User-Agent"))

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(testResponse{Name: "ok", Value: 1}))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL), WithUserAgent("schwab-agent/1.2.3"))
	var result testResponse
	err := c.doGet(context.Background(), "/test", nil, &result)

	require.NoError(t, err)
}

func TestDoRequest_NonJSONContentType_ReturnsDescriptiveError(t *testing.T) {
	// Simulates a proxy or maintenance page returning HTML instead of JSON.
	// Without Content-Type validation, this would produce a cryptic
	// json.Unmarshal error instead of telling the caller what happened.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>Service Unavailable</body></html>"))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/test", nil, &result)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected Content-Type")
	assert.Contains(t, err.Error(), "text/html")
	assert.Contains(t, err.Error(), "Service Unavailable")
}

func TestDoRequest_JSONContentTypeWithCharset_Succeeds(t *testing.T) {
	// Schwab sometimes sends "application/json;charset=UTF-8" - make sure
	// the Content-Type check doesn't reject valid JSON with charset params.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		require.NoError(t, json.NewEncoder(w).Encode(testResponse{Name: "ok", Value: 42}))
	}))
	defer srv.Close()

	c := NewClient("tok", WithBaseURL(srv.URL))
	var result testResponse
	err := c.doGet(context.Background(), "/test", nil, &result)

	require.NoError(t, err)
	assert.Equal(t, "ok", result.Name)
	assert.Equal(t, 42, result.Value)
}

func TestDoRequest_MultipleOptions(t *testing.T) {
	custom := &http.Client{Timeout: 99}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	c := NewClient("tok",
		WithBaseURL("https://custom.example.com"),
		WithHTTPClient(custom),
		WithLogger(logger),
	)

	assert.Equal(t, "https://custom.example.com", c.baseURL)
	assert.Equal(t, custom, c.httpClient)
	assert.Equal(t, logger, c.logger)
}

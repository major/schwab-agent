// Package client provides an authenticated HTTP client for the Schwab API.
//
// All requests include Bearer token authentication, JSON content headers,
// and automatic error mapping for non-2xx responses.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	schwabErrors "github.com/major/schwab-agent/internal/errors"
)

const (
	defaultBaseURL = "https://api.schwabapi.com"

	// defaultTimeout is the overall request timeout for the Schwab API client.
	// Covers the full request lifecycle: DNS, connect, TLS handshake, sending
	// the request, and reading the response. 30 seconds is generous for a REST
	// API but prevents indefinite hangs on network issues.
	defaultTimeout = 30 * time.Second
)

// Client is an authenticated HTTP client for the Schwab API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	logger     *slog.Logger
}

// Option is a functional option for NewClient.
type Option func(*Client)

// NewClient creates a new Client with the given token and options.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		token:  token,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithBaseURL sets the base URL for the client.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithHTTPClient sets the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) {
		c.logger = l
	}
}

// SetToken updates the Bearer token (used by Before hook after refresh).
func (c *Client) SetToken(token string) {
	c.token = token
}

// doRequest is the core request method that handles authentication, serialization,
// and error mapping for all HTTP methods.
func (c *Client) doRequest(ctx context.Context, method, path string, body, result any) error {
	var reqBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	// Only set Content-Type when sending a body. The Schwab API returns 400
	// on GET requests that include Content-Type: application/json.
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.logger.Debug("http request", "method", method, "path", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read the full response body for error messages or JSON decoding.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	// Map non-2xx status codes to typed errors.
	if resp.StatusCode == http.StatusUnauthorized {
		return schwabErrors.NewAuthExpiredError("authentication expired", nil)
	}
	if resp.StatusCode >= 400 {
		return schwabErrors.NewHTTPError(
			fmt.Sprintf("HTTP %d", resp.StatusCode),
			resp.StatusCode,
			string(respBody),
			nil,
		)
	}

	// Decode JSON response if a result target was provided and there is a body.
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// doGet performs a GET request with optional query parameters.
func (c *Client) doGet(ctx context.Context, path string, params map[string]string, result any) error {
	if len(params) > 0 {
		path += "?"
		first := true
		for k, v := range params {
			if !first {
				path += "&"
			}
			path += k + "=" + v
			first = false
		}
	}
	return c.doRequest(ctx, http.MethodGet, path, nil, result)
}

// doPost performs a POST request with JSON body.
func (c *Client) doPost(ctx context.Context, path string, body, result any) error {
	return c.doRequest(ctx, http.MethodPost, path, body, result)
}

// doPut performs a PUT request with JSON body.
func (c *Client) doPut(ctx context.Context, path string, body, result any) error {
	return c.doRequest(ctx, http.MethodPut, path, body, result)
}

// doDelete performs a DELETE request.
func (c *Client) doDelete(ctx context.Context, path string, result any) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, result)
}

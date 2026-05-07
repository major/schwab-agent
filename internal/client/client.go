// Package client provides the CLI's Schwab API facade.
//
// Endpoint methods route through schwab-go where that library exposes the
// behavior the CLI needs, then adapt responses back into this repository's
// stable output models and typed error hierarchy.
package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"time"

	schwab "github.com/major/schwab-go/schwab"
	"resty.dev/v3"

	"github.com/major/schwab-go/schwab/marketdata"
	"github.com/major/schwab-go/schwab/trader"

	"github.com/major/schwab-agent/internal/apperr"
)

const (
	defaultBaseURL = "https://api.schwabapi.com"

	// defaultTimeout is the overall request timeout for the Schwab API client.
	// Covers the full request lifecycle: DNS, connect, TLS handshake, sending
	// the request, and reading the response. 30 seconds is generous for a REST
	// API but prevents indefinite hangs on network issues.
	defaultTimeout = 30 * time.Second

	// maxResponseSize caps how many bytes we'll read from any API response.
	// Prevents a misbehaving server from sending a huge payload that exhausts
	// memory. 10 MB is far larger than any legitimate Schwab API response
	// (option chains with all expirations are the biggest, typically under 1 MB).
	maxResponseSize = 10 * 1024 * 1024 // 10 MB

	// defaultUserAgent identifies this client to the Schwab API. Overridden at
	// build time via WithUserAgent to include the real version from ldflags.
	defaultUserAgent = "schwab-agent/dev"

	unexpectedContentTypePreviewBytes = 200
)

// Ref holds a lazily-populated reference to a Client. Command constructors
// capture the Ref at build time; the Before hook populates it after
// authentication, so all commands share the live client via simple field
// assignment instead of the Go-unusual *x = *y dereference pattern.
type Ref struct {
	*Client

	MarketData *marketdata.Client
}

// Client is an authenticated HTTP client for the Schwab API.
type Client struct {
	baseURL   string
	resty     *resty.Client
	token     string
	tlsConfig *tls.Config
	userAgent string
	logger    *slog.Logger
}

// Option is a functional option for NewClient.
type Option func(*Client)

// NewClient creates a new Client with the given token and options.
func NewClient(token string, opts ...Option) *Client {
	rc := resty.New()
	c := &Client{
		baseURL:   defaultBaseURL,
		resty:     rc,
		token:     token,
		userAgent: defaultUserAgent,
		logger:    slog.New(slog.DiscardHandler),
	}
	rc.SetBaseURL(defaultBaseURL)
	rc.SetTimeout(defaultTimeout)
	rc.SetHeader("Accept", "application/json")
	rc.SetHeader("User-Agent", defaultUserAgent)
	rc.SetResponseBodyLimit(maxResponseSize)
	// Read c.token at request time so token refresh code can mutate the field
	// directly and the next request immediately uses the new bearer token.
	rc.AddRequestMiddleware(func(_ *resty.Client, req *resty.Request) error {
		req.SetHeader("Authorization", "Bearer "+c.token)
		return nil
	})
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) newTraderClient() *trader.Client {
	// Keep the schwab-go trader client behind this adapter so command packages
	// retain the existing internal/client boundary and typed error contract while
	// endpoint methods migrate one at a time. Build it from the current token on
	// demand because schwab-go v0.4.1 accepts WithTokenProvider but does not retain
	// it in concrete trader clients; see major/schwab-go#53.
	return trader.NewClient(
		schwab.WithToken(c.token),
		schwab.WithBaseURL(c.baseURL),
		schwab.WithResponseBodyLimit(maxResponseSize),
		schwab.WithUserAgent(c.userAgent),
		schwab.WithTLSConfig(c.tlsConfig),
	)
}

func (c *Client) newMarketDataClient() *marketdata.Client {
	// Build on demand for the same token-refresh reason as newTraderClient.
	// Keeping construction here prevents command packages from depending on
	// transport details while still letting schwab-go own request construction.
	return marketdata.NewClient(
		schwab.WithToken(c.token),
		schwab.WithBaseURL(c.baseURL),
		schwab.WithResponseBodyLimit(maxResponseSize),
		schwab.WithUserAgent(c.userAgent),
		schwab.WithTLSConfig(c.tlsConfig),
	)
}

func adaptSchwabGoModel[T any](source any) (T, error) {
	var target T
	encoded, err := json.Marshal(source)
	if err != nil {
		return target, fmt.Errorf("encode schwab-go response: %w", err)
	}
	if unmarshalErr := json.Unmarshal(encoded, &target); unmarshalErr != nil {
		return target, fmt.Errorf("decode schwab-go response into local model: %w", unmarshalErr)
	}
	return target, nil
}

func schwabAPIErrorToHTTPError(err error) error {
	var apiErr *schwab.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	if apiErr.StatusCode == http.StatusUnauthorized {
		return apperr.NewAuthExpiredError("authentication expired", err)
	}
	return apperr.NewHTTPError(
		fmt.Sprintf("HTTP %d", apiErr.StatusCode),
		apiErr.StatusCode,
		apiErr.Body,
		err,
	)
}

// WithBaseURL sets the base URL for the client.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
		c.resty.SetBaseURL(baseURL)
	}
}

// WithTLSConfig applies a custom TLS configuration to the resty client.
// Pass cfg.TLSConfig() from the auth package to enable insecure-TLS proxy support.
// A nil config is a no-op.
func WithTLSConfig(tlsCfg *tls.Config) Option {
	return func(c *Client) {
		if tlsCfg != nil {
			c.tlsConfig = tlsCfg
			c.resty.SetTLSClientConfig(tlsCfg)
		}
	}
}

// WithLogger sets the logger.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) {
		c.logger = l
	}
}

// WithUserAgent sets the User-Agent header sent with every request.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
		c.resty.SetHeader("User-Agent", ua)
	}
}

// Close releases idle connections held by the underlying resty client.
// Short-lived CLI processes can skip this since the OS reclaims resources on exit.
func (c *Client) Close() {
	_ = c.resty.Close()
}

// doRequest is the core request method that handles authentication, serialization,
// and error mapping for all HTTP methods.
func (c *Client) doRequest(ctx context.Context, method, path string, body, result any) error {
	req := c.resty.R().SetContext(ctx)
	if body != nil {
		// resty auto-sets Content-Type: application/json when SetBody is called
		// with a struct. It leaves bodyless requests alone, which preserves the
		// Schwab API quirk where GET plus Content-Type can return HTTP 400.
		req = req.SetBody(body)
	}

	resp, err := req.Execute(method, path)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}

	c.logger.DebugContext(ctx, "http request", "method", method, "path", path, "status", resp.StatusCode())

	// Map non-2xx status codes to typed errors.
	if resp.StatusCode() == http.StatusUnauthorized {
		return apperr.NewAuthExpiredError("authentication expired", nil)
	}
	if resp.StatusCode() >= http.StatusBadRequest {
		return apperr.NewHTTPError(
			fmt.Sprintf("HTTP %d", resp.StatusCode()),
			resp.StatusCode(),
			resp.String(),
			nil,
		)
	}
	if resp.Size() > maxResponseSize {
		return fmt.Errorf("execute request: %w", resty.ErrReadExceedsThresholdLimit)
	}

	// Decode JSON response if a result target was provided and there is a body.
	respBody := resp.Bytes()
	if result == nil || len(respBody) == 0 {
		return nil
	}

	// Validate Content-Type before attempting JSON decode. Without this,
	// an HTML error page from a proxy or maintenance window produces a
	// cryptic json.Unmarshal error instead of a clear diagnostic.
	ct := resp.Header().Get("Content-Type")
	if ct != "" {
		mediaType, _, parseErr := mime.ParseMediaType(ct)
		if parseErr == nil && mediaType != "application/json" {
			// Show a body preview so the caller can see what came back
			// (e.g., an HTML maintenance page or a proxy error).
			preview := resp.String()
			if len(preview) > unexpectedContentTypePreviewBytes {
				preview = preview[:unexpectedContentTypePreviewBytes] + "..."
			}
			return fmt.Errorf("unexpected Content-Type %q (expected application/json): %s", ct, preview)
		}
	}

	if unmarshalErr := json.Unmarshal(respBody, result); unmarshalErr != nil {
		return fmt.Errorf("decode response: %w", unmarshalErr)
	}

	return nil
}

// doGet performs a GET request with optional query parameters.
// Values are percent-encoded via [url.Values] to handle special characters safely.
func (c *Client) doGet(ctx context.Context, path string, params map[string]string, result any) error {
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			q.Set(k, v)
		}
		path += "?" + q.Encode()
	}
	return c.doRequest(ctx, http.MethodGet, path, nil, result)
}

// doPost performs a POST request with JSON body.
func (c *Client) doPost(ctx context.Context, path string, body, result any) error {
	return c.doRequest(ctx, http.MethodPost, path, body, result)
}

// doDelete performs a DELETE request.
func (c *Client) doDelete(ctx context.Context, path string, result any) error {
	return c.doRequest(ctx, http.MethodDelete, path, nil, result)
}

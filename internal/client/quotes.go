package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"resty.dev/v3"

	"github.com/major/schwab-agent/internal/apperr"
	"github.com/major/schwab-agent/internal/models"
	schwab "github.com/major/schwab-go/schwab"
	"github.com/major/schwab-go/schwab/marketdata"
)

// QuoteParams contains optional parameters for quote requests.
type QuoteParams struct {
	Fields     []string // Specific quote fields to return (e.g., "quote", "fundamental").
	Indicative bool     // Request indicative (non-tradeable) quotes.
}

// quoteFields returns the Schwab API fields parameter from QuoteParams.
func quoteFields(p QuoteParams) string {
	return strings.Join(p.Fields, ",")
}

// Quotes retrieves quotes for multiple symbols.
// Symbols are passed as a comma-separated query parameter.
func (c *Client) Quotes(ctx context.Context, symbols []string, p QuoteParams) (map[string]*models.QuoteEquity, error) {
	result, _, err := c.marketDataClient().GetQuotes(ctx, symbols, quoteFields(p), p.Indicative)
	if err != nil {
		return nil, mapQuoteAPIError(err, "")
	}
	return convertQuoteResponse(result)
}

// Quote retrieves a quote for a single symbol.
// Returns SymbolNotFoundError on 404.
func (c *Client) Quote(ctx context.Context, symbol string, p QuoteParams) (*models.QuoteEquity, error) {
	result, err := c.quoteResponse(ctx, symbol, p)
	if err != nil {
		return nil, mapQuoteAPIError(err, symbol)
	}
	quotes, err := convertQuoteResponse(result)
	if err != nil {
		return nil, err
	}
	q, ok := quotes[symbol]
	if !ok {
		return nil, apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), nil)
	}
	return q, nil
}

// quoteResponse calls the schwab-go endpoint that can represent the requested
// quote parameters. Schwab's indicative flag is available on the multi-symbol
// /quotes endpoint, while schwab-go's single-symbol GetQuote mirrors Schwab's
// /{symbol}/quotes endpoint and intentionally has no indicative argument.
func (c *Client) quoteResponse(ctx context.Context, symbol string, p QuoteParams) (*marketdata.QuoteResponse, error) {
	marketData := c.marketDataClient()
	if p.Indicative {
		result, _, err := marketData.GetQuotes(ctx, []string{symbol}, quoteFields(p), true)
		return result, err
	}
	return marketData.GetQuote(ctx, symbol, quoteFields(p))
}

// marketDataClient builds a schwab-go market data client for the current token.
// The root schwab-agent client stores a base API URL such as
// https://api.schwabapi.com, while schwab-go marketdata expects its base URL to
// point at the marketdata/v1 resource root. Deriving it here keeps the existing
// config semantics unchanged for callers and tests.
func (c *Client) marketDataClient() *marketdata.Client {
	return marketdata.NewClient(
		schwab.WithToken(c.token),
		schwab.WithHTTPClient(c.schwabGoHTTPClient()),
		schwab.WithBaseURL(marketDataBaseURL(c.baseURL)),
	)
}

// schwabGoHTTPClient returns a shallow copy of resty's underlying HTTP client
// with a response-safety transport inserted for schwab-go calls. The copy keeps
// timeout, proxy, and TLS behavior from resty without mutating the shared client.
func (c *Client) schwabGoHTTPClient() *http.Client {
	httpClient := *c.resty.Client()
	httpClient.Timeout = c.resty.Timeout()
	httpClient.Transport = quoteSafeTransport{base: httpClient.Transport}
	return &httpClient
}

// quoteSafeTransport restores the response safeguards that schwab-agent's resty
// doRequest path applies before JSON decoding. schwab-go intentionally owns the
// endpoint request/response parsing, but schwab-agent still needs its local
// proxy diagnostics and bounded response reads for migrated endpoints.
type quoteSafeTransport struct {
	base http.RoundTripper
}

// RoundTrip executes the request, caps the buffered body, and rejects non-JSON
// successful responses with a useful preview before schwab-go decodes them.
func (t quoteSafeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return resp, nil
	}

	body, err := readCappedResponseBody(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := validateJSONResponse(resp, body); err != nil {
		return nil, err
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return resp, nil
}

// readCappedResponseBody mirrors the 10 MB resty response limit for schwab-go.
func readCappedResponseBody(body io.ReadCloser) ([]byte, error) {
	data, readErr := io.ReadAll(io.LimitReader(body, maxResponseSize+1))
	closeErr := body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(data) > maxResponseSize {
		return nil, fmt.Errorf("execute request: %w", resty.ErrReadExceedsThresholdLimit)
	}
	return data, nil
}

// validateJSONResponse keeps the previous clear proxy/API diagnostic for HTML
// or other non-JSON successful responses. Error responses are left to schwab-go
// so APIError mapping still receives the body it expects.
func validateJSONResponse(resp *http.Response, body []byte) error {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || len(body) == 0 {
		return nil
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || mediaType == "application/json" {
		return nil
	}

	preview := string(body)
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	return fmt.Errorf("unexpected Content-Type %q (expected application/json): %s", ct, preview)
}

// marketDataBaseURL appends the Schwab market-data prefix unless the caller
// already provided a URL rooted at that prefix.
func marketDataBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/marketdata/v1") {
		return baseURL
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL + "/marketdata/v1"
	}
	return u.JoinPath("marketdata", "v1").String()
}

// convertQuoteResponse adapts schwab-go quote envelopes back into the existing
// schwab-agent model so command output and downstream command code stay stable
// while endpoint plumbing migrates to the shared library.
func convertQuoteResponse(response *marketdata.QuoteResponse) (map[string]*models.QuoteEquity, error) {
	quotes := map[string]*models.QuoteEquity{}
	if response == nil {
		return quotes, nil
	}
	for symbol, entry := range *response {
		if symbol == "errors" || entry == nil {
			continue
		}
		quote, err := convertQuoteEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("convert quote %s: %w", symbol, err)
		}
		quotes[symbol] = quote
	}
	return quotes, nil
}

// convertQuoteEntry uses JSON as a narrow compatibility bridge between the
// schwab-go quote envelope and schwab-agent's legacy model. The two structs use
// the same Schwab JSON field names, including nested quote/reference groups.
func convertQuoteEntry(entry *marketdata.QuoteEntry) (*models.QuoteEquity, error) {
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("marshal quote entry: %w", err)
	}
	var quote models.QuoteEquity
	if err := json.Unmarshal(data, &quote); err != nil {
		return nil, fmt.Errorf("decode quote entry: %w", err)
	}
	return &quote, nil
}

// mapQuoteAPIError converts schwab-go's library error into schwab-agent's typed
// error hierarchy so exit codes and JSON error envelopes remain unchanged.
func mapQuoteAPIError(err error, symbol string) error {
	var apiErr *schwab.APIError
	if !errors.As(err, &apiErr) {
		return err
	}
	if apiErr.StatusCode == http.StatusUnauthorized {
		return apperr.NewAuthExpiredError("authentication expired", err)
	}
	if apiErr.StatusCode == http.StatusNotFound && symbol != "" {
		return apperr.NewSymbolNotFoundError(fmt.Sprintf("symbol %s not found", symbol), err)
	}
	return apperr.NewHTTPError(
		fmt.Sprintf("HTTP %d", apiErr.StatusCode),
		apiErr.StatusCode,
		apiErr.Message,
		err,
	)
}

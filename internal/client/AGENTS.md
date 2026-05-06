# AGENTS.md - internal/client

> Leave generous comments when fixing bugs or working around API quirks. Anything that might save a future developer from re-discovering the same issue is worth writing down.

HTTP client for the Charles Schwab API. Wraps all API endpoints with typed Go methods. Migrated endpoints should prefer `github.com/major/schwab-go` adapters while preserving schwab-agent's public method signatures, JSON output models, and typed error hierarchy.

## Client Construction

Functional options pattern:

```go
c := client.NewClient("bearer-token",
    client.WithBaseURL(url),
    client.WithTLSConfig(tlsConfig),
    client.WithLogger(logger),
)
```

Production code injects the client via the Before hook in `main.go`. Tests use `client.NewClient("test-token", client.WithBaseURL(server.URL))`.

WithTLSConfig applies a custom TLS configuration to the resty client's transport (e.g., InsecureSkipVerify for local proxy setups). Pass nil for default TLS behavior. The resty client handles connection pooling and lifecycle; call c.Close() to release idle connections. Close() is wired via an After hook in main.go.

## Adding a New Endpoint

1. Create `<resource>.go` with methods on `*Client`
2. Use the appropriate HTTP helper: `doGet`, `doPost`, `doDelete`, or call `doRequest` directly when the endpoint needs a method-specific response header or status-code quirk
3. Create `<resource>_test.go` with httptest-based tests
4. Define any new request/response types in `internal/models/`

## HTTP Helpers

Core method `doRequest` uses resty v3 internally. It sets the Bearer token via request middleware (reads c.token at request time for token refresh support), validates Content-Type before JSON decoding, and maps status codes to typed errors. Migrated `schwab-go` adapters should reuse `c.resty.Client()` so timeout/TLS transport behavior stays consistent, then map library errors back to `internal/apperr`. Thin wrappers:

- `doGet(ctx, path, params, result)`: GET with query params
- `doPost(ctx, path, body, result)`: POST with JSON body
- `doDelete(ctx, path, result)`: DELETE

Content-Type header is set by resty only when a request body is present (not on GET). Accept: application/json is set globally on the resty client.

## Error Mapping

`doRequest` maps HTTP status codes to typed errors:

| Status | Error Type |
|---|---|
| 401 | `AuthExpiredError` |
| 400, 422 (on order endpoints) | `OrderRejectedError` |
| Other 4xx/5xx | `HTTPError` (includes status code + body) |

The `PlaceOrder` method has custom status handling (bypasses `doRequest`) to extract the order ID from the Location header and map 400/422 to `OrderRejectedError`.

## Endpoint Methods

Each file maps to one Schwab API resource:

| File | Methods | API Path Prefix |
|---|---|---|
| accounts.go | `Accounts()`, `Account()` | `/trader/v1/accounts` |
| chains.go | `Chains()` | `/marketdata/v1/chains` |
| history.go | `PriceHistory()` | `/marketdata/v1/pricehistory` |
| instruments.go | `Instruments()` | `/marketdata/v1/instruments` |
| markets.go | `MarketHours()` | `/marketdata/v1/markets` |
| movers.go | `Movers()` | `/marketdata/v1/movers` |
| orders.go | `ListOrders()`, `AllOrders()`, `GetOrder()`, `PlaceOrder()`, `PreviewOrder()`, `ReplaceOrder()`, `CancelOrder()` | `/trader/v1/accounts/{hash}/orders` |
| preferences.go | `UserPreference()` | `/trader/v1/userPreference` |
| quotes.go | `Quote()`, `Quotes()` | `/marketdata/v1/quotes` via schwab-go marketdata |
| transactions.go | `Transactions()`, `Transaction()` | `/trader/v1/accounts/{hash}/transactions` |

## Query Parameters

Methods accepting filters use either:

- `map[string]string` passed to `doGet` for legacy direct-resty endpoints
- Typed param structs with `toQueryParams()` method (e.g., `OrderListParams`, `ChainParams`)
- Dedicated parameter conversion helpers for `schwab-go` adapters when the shared library uses typed arguments instead of a query map

## Error Conversion in Client Methods

Some methods convert generic errors to domain-specific ones:

```go
// quotes.go: schwab-go APIError -> SchwabError subtype
var httpErr *apperr.HTTPError
if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
    return nil, apperr.NewSymbolNotFoundError(...)
}
```

## Testing Pattern

Every endpoint file has a corresponding `_test.go` using httptest:

```go
func TestQuote(t *testing.T) {
    // Arrange
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Validate request (method, path, headers, body)
        // Write canned JSON response
    }))
    defer server.Close()
    c := client.NewClient("test-token", client.WithBaseURL(server.URL))

    // Act
    result, err := c.Quote(context.Background(), "AAPL")

    // Assert
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

Tests validate request shape inline (headers, method, path, body) and assert response parsing. Error paths test status code mapping to typed errors via `require.ErrorAs()`.

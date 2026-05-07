# AGENTS.md - internal/client

> Leave generous comments when fixing bugs or working around API quirks. Anything that might save a future developer from re-discovering the same issue is worth writing down.

Compatibility facade for Schwab API access. Route endpoint methods through `github.com/major/schwab-go` when that library preserves this CLI's output and error contracts. Keep local compatibility decoders only for documented schwab-go gaps. 16 Go files (8 source + 8 test).

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

WithTLSConfig applies a custom TLS configuration to both schwab-go clients and the remaining resty compatibility transport (e.g., InsecureSkipVerify for local proxy setups). Pass nil for default TLS behavior. The resty client handles connection pooling for compatibility calls; call c.Close() to release idle connections. Close() is wired via an After hook in main.go.

## Adding a New Endpoint

1. Create `<resource>.go` with methods on `*Client`
2. Prefer the appropriate schwab-go trader or marketdata method, then adapt back into `internal/models` only when output parity is proven by tests
3. Use `doGet`, `doPost`, or direct resty only for documented schwab-go gaps such as response headers, incompatible request/response models, or missing optional filter behavior
4. Create `<resource>_test.go` with httptest-based tests
5. Define any new request/response types in `internal/models/`

## HTTP Helpers

Core method `doRequest` is the remaining resty v3 compatibility path. It sets the Bearer token via request middleware (reads c.token at request time for token refresh support), validates Content-Type before JSON decoding, and maps status codes to typed errors. Thin wrappers:

- `doGet(ctx, path, params, result)`: GET with query params
- `doPost(ctx, path, body, result)`: POST with JSON body

Content-Type header is set by resty only when a request body is present (not on GET). Accept: application/json is set globally on the resty client. New migrated calls should use `newTraderClient()` or `newMarketDataClient()` so schwab-go owns request construction.

## Error Mapping

`doRequest` maps HTTP status codes to typed errors:

| Status | Error Type |
|---|---|
| 401 | `AuthExpiredError` |
| Other 4xx/5xx | `HTTPError` (includes status code + body) |

The `PlaceOrder` and `ReplaceOrder` methods have custom status handling (bypasses `doRequest`) to extract the order ID from the Location header and map 400/422 to `OrderRejectedError`. Keep this until schwab-go exposes order mutation response headers or order IDs.

## Endpoint Methods

Each file maps to one Schwab API resource:

| File | Methods | API Path Prefix |
|---|---|---|
| accounts.go | `AccountNumbers()` via schwab-go; `Accounts()`, `Account()` via schwab-go raw responses plus local model decoder | `/trader/v1/accounts` |
| chains.go | `ExpirationChainForSymbol()` via schwab-go; `OptionChain()` compatibility decoder pending major/schwab-go#62 | `/marketdata/v1/chains`, `/marketdata/v1/expirationchain` |
| orders.go | `ListOrders()`, `AllOrders()`, `GetOrder()`, `CancelOrder()` via schwab-go; `PlaceOrder()`, `PreviewOrder()`, `ReplaceOrder()` compatibility paths pending major/schwab-go#65 | `/trader/v1/accounts/{hash}/orders` |
| preferences.go | `UserPreference()` compatibility decoder pending major/schwab-go#63 | `/trader/v1/userPreference` |
| quotes.go | `Quote()` via schwab-go | `/marketdata/v1/quotes` |
| transactions.go | `Transactions()`, `Transaction()` compatibility decoder pending major/schwab-go#64 | `/trader/v1/accounts/{hash}/transactions` |

`client.go` contains shared client construction and HTTP helpers. `params.go` contains reusable query parameter helpers, not endpoint methods.

## Query Parameters

Methods accepting filters use either:

- `map[string]string` passed to `doGet` for remaining compatibility decoders
- Typed param structs or schwab-go parameter structs (e.g., `OrderListParams`, `TransactionListParams`, `marketdata.OptionChainParams`)

## Error Conversion in Client Methods

Some methods convert generic errors to domain-specific ones:

```go
// quotes.go: 404 HTTPError -> SymbolNotFoundError
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

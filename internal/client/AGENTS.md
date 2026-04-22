# AGENTS.md - internal/client

> Leave generous comments when fixing bugs or working around API quirks. Anything that might save a future developer from re-discovering the same issue is worth writing down.

HTTP client for the Charles Schwab API. Wraps all API endpoints with typed Go methods. 22 files (11 source + 11 test).

## Client Construction

Functional options pattern:

```go
c := client.NewClient("bearer-token",
    client.WithBaseURL(url),
    client.WithHTTPClient(httpClient),
    client.WithLogger(logger),
)
```

Production code injects the client via the Before hook in `main.go`. Tests use `client.NewClient("test-token", client.WithBaseURL(server.URL))`.

## Adding a New Endpoint

1. Create `<resource>.go` with methods on `*Client`
2. Use the appropriate HTTP helper: `doGet`, `doPost`, `doPut`, `doDelete`
3. Create `<resource>_test.go` with httptest-based tests
4. Define any new request/response types in `internal/models/`

## HTTP Helpers

Core method `doRequest` handles auth headers, content type, status code mapping, and JSON decoding. Thin wrappers:

- `doGet(ctx, path, params, result)`: GET with query params
- `doPost(ctx, path, body, result)`: POST with JSON body
- `doPut(ctx, path, body, result)`: PUT with JSON body
- `doDelete(ctx, path, result)`: DELETE

Content-Type header is set only on non-GET requests. Accept header always set.

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
| quotes.go | `Quote()`, `Quotes()` | `/marketdata/v1/quotes` |
| transactions.go | `Transactions()`, `Transaction()` | `/trader/v1/accounts/{hash}/transactions` |

## Query Parameters

Methods accepting filters use either:

- `map[string]string` passed to `doGet` (simple cases like `quotes.go`)
- Typed param structs with `toQueryParams()` method (e.g., `OrderListParams`, `ChainParams`)

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

# AGENTS.md - internal/auth

> Leave generous comments when fixing bugs or working around API quirks. Anything that might save a future developer from re-discovering the same issue is worth writing down.

OAuth2 flow, token lifecycle, and config management for Schwab API authentication.

## Token Exchange

`ExchangeCode()` and `RefreshAccessToken()` use resty v3 via the `newOAuthClient` helper. Both create a short-lived resty client with `defer client.Close()`. Requests are form-urlencoded POSTs with HTTP Basic Auth (client ID + secret). The token endpoint URL is derived from `Config.BaseURL`.

`newOAuthClient` applies `Config.TLSConfig()` so insecure proxy setups work for token requests the same way they do for API requests.

## TLSConfig

`Config.TLSConfig()` returns `*tls.Config`:

- Returns nil when `base_url_insecure` is false (default TLS behavior).
- Returns `&tls.Config{InsecureSkipVerify: true}` when `base_url_insecure` is true.

Both the auth token exchange (`newOAuthClient`) and the API client (`client.WithTLSConfig`) call this method, so insecure mode is applied consistently across all outbound connections.

## Callback Server

`StartCallbackServer` and `startCallbackServer` use raw `net/http` - this is intentional. The callback server is inbound server-side code that receives Schwab's OAuth redirect, not an outbound HTTP client. It is not a resty migration candidate.

## Config

JSON at `~/.config/schwab-agent/config.json`. Fields: `client_id`, `client_secret`, `base_url`, `base_url_insecure`, `callback_url`, `default_account`, `i-also-like-to-live-dangerously`. Env vars (`SCHWAB_CLIENT_ID`, `SCHWAB_CLIENT_SECRET`, `SCHWAB_BASE_URL`, `SCHWAB_BASE_URL_INSECURE`, `SCHWAB_CALLBACK_URL`) override file values.

## Testing

- `httptest.NewServer` for plain HTTP token endpoint mocking.
- `httptest.NewTLSServer` for TLS token endpoint mocking (tests the insecure path).
- `sendCallbackRequest` helper uses a raw `http.Client` - this is test infrastructure for exercising the callback server, not production code.

# AGENTS.md - internal/auth

> Leave generous comments when fixing bugs or working around API quirks. Anything that might save a future developer from re-discovering the same issue is worth writing down.

OAuth2 config adapters and token lifecycle helpers for Schwab API authentication. Generic OAuth mechanics should live in `github.com/major/schwab-go/schwab/auth`; this package keeps schwab-agent-specific config, output, and compatibility behavior.

## Token Exchange

`ExchangeCode()`, `RefreshAccessToken()`, `RunLogin()`, token load/save, authorization URL construction, and the HTTPS callback server delegate to `github.com/major/schwab-go/schwab/auth`. Keep this package as a thin adapter around schwab-go plus app-specific error mapping.

`Config.schwabAuthConfig(tokenEndpoint)` adapts schwab-agent config to schwab-go auth config. Production code derives the OAuth base URL from `Config.APIBaseURL()`. Tests may pass an injected token endpoint, which is normalized to an OAuth base URL by trimming a trailing `/token`.

## TLSConfig

`Config.TLSConfig()` returns `*tls.Config`:

- Returns nil when `base_url_insecure` is false (default TLS behavior).
- Returns `&tls.Config{InsecureSkipVerify: true}` when `base_url_insecure` is true.

Both auth token exchange/refresh (`oauthHTTPClient`) and the API client (`client.WithTLSConfig`) call this method, so insecure mode is applied consistently across all outbound connections.

## Callback Server

`StartCallbackServer` wraps `schwab-go/schwab/auth.StartCallbackServer`. Do not re-add local certificate generation, callback handlers, or `sync.Once` request processing unless schwab-go cannot support a required behavior.

## Config

JSON at `~/.config/schwab-agent/config.json`. Fields: `client_id`, `client_secret`, `base_url`, `base_url_insecure`, `callback_url`, `default_account`, `i-also-like-to-live-dangerously`. Env vars (`SCHWAB_CLIENT_ID`, `SCHWAB_CLIENT_SECRET`, `SCHWAB_BASE_URL`, `SCHWAB_BASE_URL_INSECURE`, `SCHWAB_CALLBACK_URL`) override file values.

## Testing

- Use `httptest.NewTLSServer` for token endpoint mocking and set `BaseURLInsecure: true`; schwab-go auth config intentionally requires HTTPS OAuth URLs.
- `sendCallbackRequest` helper uses a raw `http.Client` - this is test infrastructure for exercising the callback server, not production code.

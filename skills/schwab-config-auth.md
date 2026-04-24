# Configuration and Authentication

## Auth Commands

```bash
schwab-agent auth login              # Start OAuth2 flow (opens browser, stores tokens)
schwab-agent auth login --no-browser # Print auth URL instead of opening browser
schwab-agent auth status             # Check token and refresh token expiration
schwab-agent auth refresh            # Force token refresh without re-authenticating
```

Auth commands skip the global auth check (no existing token needed to run them).

## Configuration

Config file: `~/.config/schwab-agent/config.json`

| Field | Description |
|-------|-------------|
| `client_id` | Schwab app client ID |
| `client_secret` | Schwab app client secret |
| `base_url` | Outbound base URL for REST API and OAuth requests (default: `https://api.schwabapi.com`). Derives authorize, token, and API endpoints. |
| `base_url_insecure` | Skip TLS verification for outbound calls (for local proxies with self-signed certs) |
| `callback_url` | OAuth2 callback URL (default: `https://127.0.0.1:8182`) |
| `default_account` | Default account hash for commands that need an account |
| `i-also-like-to-live-dangerously` | Enable mutable operations (order place/cancel/replace) |

Set a default account: `schwab-agent account set-default <hash-value>`

## Environment Variables

Override config file values (higher priority).

| Variable | Overrides |
|----------|-----------|
| `SCHWAB_CLIENT_ID` | `client_id` |
| `SCHWAB_CLIENT_SECRET` | `client_secret` |
| `SCHWAB_BASE_URL` | `base_url` |
| `SCHWAB_BASE_URL_INSECURE` | `base_url_insecure` |
| `SCHWAB_CALLBACK_URL` | `callback_url` |

## Troubleshooting

| Problem | Fix |
|---------|-----|
| "auth required" | Run `schwab-agent auth login` |
| "auth expired" | Refresh token stale (>6.5 days), run `auth login` again |
| "callback error" | Verify callback URL matches Schwab app settings |
| TLS/certificate error with proxy | Set `base_url` to proxy URL, enable `base_url_insecure` |
| Missing credentials | Set `SCHWAB_CLIENT_ID` and `SCHWAB_CLIENT_SECRET` env vars, or add to config.json |

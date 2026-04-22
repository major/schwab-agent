# Configuration and Authentication

schwab-agent uses OAuth2 to authenticate with the Schwab API. Credentials are stored in a local config file and tokens are refreshed automatically.

## Setup

Run the interactive setup to store your Schwab app credentials:

```bash
schwab-agent auth setup
```

This creates `~/.config/schwab-agent/config.json` with your client ID, client secret, and callback URL.

## Login

Start the OAuth2 flow. Opens a browser for Schwab authentication, receives the callback, and stores tokens locally. Tokens are refreshed automatically before expiration.

```bash
schwab-agent auth login
```

## Status

Check current token expiration and refresh token staleness:

```bash
schwab-agent auth status
```

## Configuration

Config file location: `~/.config/schwab-agent/config.json`

| Field | Description |
|-------|-------------|
| `client_id` | Schwab app client ID |
| `client_secret` | Schwab app client secret |
| `callback_url` | OAuth2 callback URL (default: https://127.0.0.1:8182) |
| `default_account` | Default account hash for commands that need an account |
| `i-also-like-to-live-dangerously` | Set to true to enable mutable operations (order placement, cancel, replace) |

Default callback URL: `https://127.0.0.1:8182`

Set a default account to avoid passing `--account` on every command:

```bash
schwab-agent account set-default <hash-value>
```

## Environment Variables

Environment variables take priority over config file values.

| Variable | Description |
|----------|-------------|
| `SCHWAB_CLIENT_ID` | Override client ID from config file |
| `SCHWAB_CLIENT_SECRET` | Override client secret from config file |
| `SCHWAB_CALLBACK_URL` | Override callback URL (default: https://127.0.0.1:8182) |

## Troubleshooting

| Problem | Fix |
|---------|-----|
| "auth required" error | Run `schwab-agent auth login` to get new tokens |
| "auth expired" error | Refresh token is stale (>6.5 days old), run `schwab-agent auth login` again |
| "callback error" | Check callback URL matches Schwab app settings (default: https://127.0.0.1:8182) |
| Missing credentials | Run `schwab-agent auth setup` or set SCHWAB_CLIENT_ID and SCHWAB_CLIENT_SECRET env vars |

# schwab-agent

[![CI](https://github.com/major/schwab-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/major/schwab-agent/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/major/schwab-agent)](https://go.dev/)
[![codecov](https://codecov.io/gh/major/schwab-agent/branch/main/graph/badge.svg)](https://codecov.io/gh/major/schwab-agent)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/major/schwab-agent/badge)](https://scorecard.dev/viewer/?uri=github.com/major/schwab-agent)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

CLI tool for AI agents to trade via the Charles Schwab API. Single binary, JSON-first output, built-in safety guards.

> **Disclaimer:** This project is not affiliated with, endorsed by, or sponsored by Charles Schwab & Co., Inc. or any of its subsidiaries. "Schwab" and "thinkorswim" are trademarks of Charles Schwab & Co., Inc. This is an independent, open-source tool that uses Schwab's publicly available APIs.

## Why

AI agents need a reliable way to interact with brokerage APIs. schwab-agent wraps the Schwab Retail Trader and Market Data APIs behind a straightforward CLI with structured JSON output that agents can parse without guessing.

## Install

### From source

```bash
go install github.com/major/schwab-agent/cmd/schwab-agent@latest
```

### From releases

Pre-built binaries for Linux and macOS (amd64/arm64) are available on the [releases page](https://github.com/major/schwab-agent/releases).

### Build locally

```bash
git clone https://github.com/major/schwab-agent.git
cd schwab-agent
make build
```

## Getting started

### 1. Create a Schwab developer app

Register at [developer.schwab.com](https://developer.schwab.com/) and create an app. You'll need your client ID and client secret.

### 2. Configure

Create `~/.config/schwab-agent/config.json`:

```json
{
  "client_id": "your-client-id",
  "client_secret": "your-client-secret",
  "base_url": "https://api.schwabapi.com",
  "base_url_insecure": false,
  "callback_url": "https://127.0.0.1:8182"
}
```

`base_url` is the single outbound endpoint root for REST API calls and Schwab OAuth authorize/token requests. Leave it at the default for direct Schwab access, or point it at a Schwab-compatible proxy.

If your proxy terminates TLS with a local self-signed certificate, set `base_url_insecure` to `true` so outbound REST and OAuth token/refresh requests skip certificate verification. This is intentionally opt-in for local development only.

Or use environment variables (these take priority over the config file):

```bash
export SCHWAB_CLIENT_ID=your-client-id
export SCHWAB_CLIENT_SECRET=your-client-secret
export SCHWAB_BASE_URL=https://api.schwabapi.com   # default
export SCHWAB_BASE_URL_INSECURE=false              # default
export SCHWAB_CALLBACK_URL=https://127.0.0.1:8182  # default
```

### 3. Log in

```bash
schwab-agent auth login
```

Opens your browser for Schwab's OAuth2 flow. After authorization, tokens are stored locally and refreshed automatically.

### 4. Check status

```bash
schwab-agent auth status
```

## Usage

Every command returns structured JSON. Success responses use a `{"data": ..., "metadata": ...}` envelope. Errors use `{"error": {"code": ..., "message": ..., "details": ...}}`.

For full command reference with examples, see the [skill files](skills/):

- [Configuration and auth](skills/schwab-config-auth.md)
- [Read-only commands](skills/schwab-read.md) (quotes, chains, history, accounts, instruments)
- [Trading workflow](skills/schwab-trade.md) (safety, order placement, cancellation)
- [Order builder](skills/schwab-order-builder.md) (equity, option, bracket, OCO, spreads)
- [Technical analysis](skills/schwab-ta.md) (SMA, EMA, RSI, MACD, ATR, BBands, Stoch, ADX, VWAP, HV, expected-move)

### Quick examples

```bash
# Get a quote
schwab-agent quote get AAPL

# List accounts
schwab-agent account list

# Get option chains
schwab-agent chain get AAPL

# Place a limit order (requires safety config + --confirm)
schwab-agent order place equity \
  --symbol AAPL \
  --action BUY \
  --quantity 10 \
  --order-type LIMIT \
  --price 150.00 \
  --duration DAY \
  --confirm

# Preview an order without placing it
schwab-agent order preview equity \
  --symbol AAPL \
  --action BUY \
  --quantity 10 \
  --order-type MARKET

# Build order JSON offline
schwab-agent order build equity \
  --symbol AAPL \
  --action BUY \
  --quantity 10 \
  --order-type LIMIT \
  --price 150.00 \
  --duration DAY

# Place from a JSON spec
schwab-agent order place --spec @order.json --confirm
```

## Commands

| Command | Description |
|---|---|
| `auth` | Login, status, token refresh |
| `account` | List (with nicknames), get details, set default, transactions |
| `position` | List positions for one or all accounts (with computed cost basis and P&L) |
| `quote` | Get quotes for one or more symbols |
| `order` | List, get, place, preview, build, cancel, replace |
| `chain` | Option chain data (`get`, `expiration`) |
| `history` | Price history for a symbol |
| `ta` | Technical analysis (sma, ema, rsi, macd, atr, bbands, stoch, adx, vwap, hv, expected-move) |
| `market` | Market hours and top movers |
| `symbol` | Build and parse OCC option symbols (no auth required) |
| `instrument` | Search instruments |
| `schema` | CLI schema introspection (auto-generated) |

### Global flags

```text
--account   Override the default account for this command
--verbose   Enable verbose logging
--config    Path to config file (default: ~/.config/schwab-agent/config.json)
--token     Path to token file (default: ~/.config/schwab-agent/token.json)
```

## Safety

schwab-agent has two layers of protection for operations that modify your account:

1. **Config flag**: Set `"i-also-like-to-live-dangerously": true` in your config file to enable mutable operations (order placement, cancellation, replacement).
2. **CLI flag**: Pass `--confirm` on each mutable command.

Both are required. This prevents accidental trades from misconfigured agents.

## Agent integration

schwab-agent ships with [skill files](skills/SKILL.md) that teach AI agents how to use the CLI. These cover command reference, examples, JSON output format, and common recipes.

The `schema` command provides machine-readable CLI introspection:

```bash
schwab-agent schema
```

## Development

```bash
make build      # Build binary
make test       # Run tests (race detector + coverage)
make lint       # Run golangci-lint
make install    # Install to /usr/local/bin
make clean      # Remove binary
```

Tests use `httptest.NewServer()` for API mocking, testify for assertions, and `t.TempDir()` for file I/O. No external services needed.

### Project layout

```text
cmd/schwab-agent/       Entry point, command tree construction
internal/
  auth/                 OAuth2 flow, token lifecycle, config
  client/               Schwab API HTTP client
  commands/             CLI command handlers
  errors/               Typed error hierarchy with exit codes
  models/               API data structures
  orderbuilder/         Order construction and validation
  output/               JSON envelope output
skills/                 Agent skill files (plain markdown)
```

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Validation or order build error |
| 2 | Symbol or account not found |
| 3 | Authentication required, expired, or failed |
| 4 | HTTP error from Schwab API |
| 5 | Order rejected |

## License

[MIT](LICENSE)

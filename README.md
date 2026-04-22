# schwab-agent

[![CI](https://github.com/major/schwab-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/major/schwab-agent/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/major/schwab-agent)](https://go.dev/)
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

```bash
schwab-agent auth setup
```

This creates `~/.config/schwab-agent/config.json` with your credentials. You can also use environment variables:

```bash
export SCHWAB_CLIENT_ID=your-client-id
export SCHWAB_CLIENT_SECRET=your-client-secret
export SCHWAB_CALLBACK_URL=https://127.0.0.1:8182  # default
```

Environment variables take priority over the config file.

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

### Get a quote

```bash
schwab-agent quote get AAPL
```

### List accounts

```bash
schwab-agent account list
```

### Get option chains

```bash
schwab-agent chain AAPL
```

### Place an order

Orders require two safety checks: the `i-also-like-to-live-dangerously` config flag and the `--confirm` CLI flag.

```bash
# Enable mutable operations in config first
schwab-agent order place equity \
  --symbol AAPL \
  --instruction BUY \
  --quantity 10 \
  --order-type LIMIT \
  --price 150.00 \
  --duration DAY \
  --confirm
```

### Preview before placing

```bash
schwab-agent order preview equity \
  --symbol AAPL \
  --instruction BUY \
  --quantity 10 \
  --order-type MARKET
```

### Build order JSON offline

```bash
schwab-agent order build equity \
  --symbol AAPL \
  --instruction BUY \
  --quantity 10 \
  --order-type LIMIT \
  --price 150.00 \
  --duration DAY
```

### Use a JSON spec directly

```bash
schwab-agent order place --spec '{"orderType": "LIMIT", ...}' --confirm

# Or from a file
schwab-agent order place --spec @order.json --confirm

# Or from stdin
cat order.json | schwab-agent order place --spec - --confirm
```

### Technical analysis

The `ta` command calculates indicators from Schwab price history data. All subcommands take a symbol as the first argument and return a JSON envelope with timestamped values.

```bash
schwab-agent ta sma AAPL
schwab-agent ta rsi AAPL --period 14 --interval daily --points 10
```

#### Shared flags

All `ta` subcommands accept these flags:

| Flag | Type | Default | Description |
|---|---|---|---|
| `--interval` | string | `daily` | Data interval: `daily`, `weekly`, `1min`, `5min`, `15min`, `30min` |
| `--points` | int | `0` | Number of most-recent output points to return (0 = all) |

#### sma - Simple Moving Average

Arithmetic mean of closing prices over a rolling window.

```bash
schwab-agent ta sma AAPL --period 20
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--period` | int | `20` | Lookback period |

#### ema - Exponential Moving Average

Like SMA but weights recent prices more heavily.

```bash
schwab-agent ta ema AAPL --period 20
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--period` | int | `20` | Lookback period |

#### rsi - Relative Strength Index

Momentum oscillator (0-100) measuring speed and magnitude of price changes.

```bash
schwab-agent ta rsi AAPL --period 14
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--period` | int | `14` | Lookback period |

#### macd - Moving Average Convergence/Divergence

Trend-following indicator using the difference between two EMAs, plus a signal line and histogram.

```bash
schwab-agent ta macd AAPL --fast 12 --slow 26 --signal 9
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--fast` | int | `12` | Fast EMA period |
| `--slow` | int | `26` | Slow EMA period |
| `--signal` | int | `9` | Signal EMA period |

#### atr - Average True Range

Volatility indicator measuring average price range over a period.

```bash
schwab-agent ta atr AAPL --period 14
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--period` | int | `14` | Lookback period |

#### bbands - Bollinger Bands

Volatility bands placed above and below a moving average at a configurable number of standard deviations.

```bash
schwab-agent ta bbands AAPL --period 20 --std-dev 2.0
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--period` | int | `20` | Lookback period |
| `--std-dev` | float | `2.0` | Standard deviations for band width |

#### stoch - Stochastic Oscillator

Momentum indicator comparing a closing price to its range over a lookback period.

```bash
schwab-agent ta stoch AAPL --k-period 14 --smooth-k 3 --d-period 3
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--k-period` | int | `14` | Fast %K lookback period |
| `--smooth-k` | int | `3` | Slow %K smoothing period |
| `--d-period` | int | `3` | Slow %D period |

#### adx - Average Directional Index

Trend strength indicator (0-100) with directional components (+DI and -DI).

```bash
schwab-agent ta adx AAPL --period 14
```

| Flag | Type | Default | Description |
|---|---|---|---|
| `--period` | int | `14` | Lookback period |

#### Output format

All `ta` subcommands return the same envelope shape. Single-value indicators (sma, ema, rsi, atr) use a field named after the indicator:

```json
{
  "data": {
    "indicator": "sma",
    "symbol": "AAPL",
    "interval": "daily",
    "period": 20,
    "values": [
      {"datetime": "2024-01-15T00:00:00Z", "sma": 150.25},
      {"datetime": "2024-01-16T00:00:00Z", "sma": 151.10}
    ]
  },
  "metadata": {"timestamp": "2024-01-17T12:00:00Z"}
}
```

Multi-value indicators use indicator-specific fields per value entry:

| Indicator | Value fields |
|---|---|
| `macd` | `macd`, `signal`, `histogram` |
| `bbands` | `upper`, `middle`, `lower` |
| `stoch` | `slowk`, `slowd` |
| `adx` | `adx`, `plus_di`, `minus_di` |

#### Recipes

| Goal | Command |
|---|---|
| 20-day SMA on daily closes | `schwab-agent ta sma AAPL` |
| RSI with last 5 readings | `schwab-agent ta rsi AAPL --points 5` |
| MACD on 5-minute bars | `schwab-agent ta macd AAPL --interval 5min` |
| Bollinger Bands with tighter bands | `schwab-agent ta bbands AAPL --std-dev 1.5` |
| Stochastic on weekly data | `schwab-agent ta stoch AAPL --interval weekly` |
| ADX trend strength check | `schwab-agent ta adx AAPL --points 1` |

## Commands

| Command | Description |
|---|---|
| `auth` | Setup, login, status, token refresh |
| `account` | List (with nicknames), get details, set default, transactions |
| `quote` | Get quotes for one or more symbols |
| `order` | List, get, place, preview, build, cancel, replace |
| `chain` | Option chain data |
| `history` | Price history for a symbol |
| `ta` | Technical analysis indicators (sma, ema, rsi, macd, atr, bbands, stoch, adx) |
| `market` | Market hours and top movers |
| `symbol` | Build and parse OCC option symbols (no auth required) |
| `instrument` | Search instruments |
| `skills` | Generate agent skill files |
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

## Configuration

Config lives at `~/.config/schwab-agent/config.json`:

```json
{
  "client_id": "your-client-id",
  "client_secret": "your-client-secret",
  "callback_url": "https://127.0.0.1:8182",
  "default_account": "12345678",
  "i-also-like-to-live-dangerously": false
}
```

Set a default account to avoid passing `--account` on every command:

```bash
schwab-agent account set-default 12345678
```

## Agent integration

schwab-agent generates skill files that teach AI agents how to use the CLI:

```bash
schwab-agent skills
```

This writes markdown files to `./skills/` with command reference, examples, and the JSON output format.

The `schema` command provides machine-readable CLI introspection:

```bash
schwab-agent schema
```

## Development

```bash
make build      # Build binary
make test       # Run tests (race detector + coverage)
make lint       # Run golangci-lint
make skills     # Generate skill files
make install    # Install to /usr/local/bin
make clean      # Remove binary
```

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
skills/                 Generated skill files
```

### Running tests

```bash
make test
```

Tests use `httptest.NewServer()` for API mocking, testify for assertions, and `t.TempDir()` for file I/O. No external services needed.

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

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

Every command returns structured JSON. Success responses use a `{"data": ..., "metadata": ...}` envelope. Errors use structcli's top-level `StructuredError` shape, for example `{"error":"unknown_flag","exit_code":12,"message":"unknown flag: --bogus","command":"schwab-agent quote get","flag":"bogus"}`. Domain errors use the same shape, with remediation text in `hint` when available.

Run any command with `--help` to see detailed usage, examples, and available flags.

### Quick examples

```bash
# Get a stock quote
schwab-agent quote get AAPL

# Get an option quote by contract details (instead of OCC symbol)
schwab-agent quote get --underlying AAPL --expiration 2025-06-20 --strike 200 --call

# List compact account identifiers for agent workflows
schwab-agent account summary

# List compact account identifiers and holdings in one response
schwab-agent account summary --positions

# List full account details and balances
schwab-agent account list

# Get option chains
schwab-agent chain get AAPL

# Get a compact option planning ticket
schwab-agent option ticket get AAPL --expiration <YYYY-MM-DD> --strike 200 --call

# Get the latest row for multiple moving averages in one technical-analysis run.
# Technical-analysis output is always keyed by symbol under data.<SYMBOL>.
schwab-agent ta sma AAPL --period 21,50,200

# Batch the latest technical indicator row across several symbols
schwab-agent ta atr AAPL MSFT NVDA

# Get a compact technical-analysis dashboard in one history fetch
schwab-agent ta dashboard AAPL

# Place a limit order (requires safety config)
schwab-agent order place equity \
  --symbol AAPL \
  --action BUY \
  --quantity 10 \
  --type LIMIT \
  --price 150.00 \
  --duration DAY

# Preview an order without placing it, then save the exact reviewed payload
schwab-agent order preview equity \
  --symbol AAPL \
  --action BUY \
  --quantity 10 \
  --type MARKET \
  --save-preview

# Place the exact payload returned by previewDigest.digest
schwab-agent order place --from-preview <digest>

# Show recent order activity, including filled/canceled/replaced orders
schwab-agent order list --recent

# Build order JSON offline
schwab-agent order build equity \
  --symbol AAPL \
  --action BUY \
  --quantity 10 \
  --type LIMIT \
  --price 150.00 \
  --duration DAY

# Use flag aliases (--instruction for --action, --order-type for --type)
schwab-agent order place equity \
  --symbol AAPL \
  --instruction BUY \
  --quantity 10 \
  --order-type LIMIT \
  --price 150.00 \
  --duration DAY

# Replace an option order by contract details
schwab-agent order replace option 123456789 \
  --underlying AAPL \
  --expiration 2025-06-20 \
  --strike 200 \
  --call \
  --action BUY_TO_OPEN \
  --quantity 5 \
  --type LIMIT \
  --price 3.50 \
  --duration DAY

# Use account nickname or number instead of hash
schwab-agent account get -a "My Roth IRA"
schwab-agent position list -a 12345678

# Filter holdings for portfolio triage
schwab-agent position list --all-accounts --symbol AAPL --symbol MSFT --sort value-desc
schwab-agent position list --losers-only --min-pnl -500 --sort pnl-asc

# Place from a JSON spec
schwab-agent order place --spec @order.json
```

## Commands

| Command | Description |
|---|---|
| `auth` | Login, status, token refresh |
| `account` | Compact summaries, full account details, hashes, set default, transactions |
| `position` | List, filter, and sort positions for one or all accounts (with computed cost basis and P&L) |
| `quote` | Get quotes for one or more symbols (supports structured option flags) |
| `order` | List, get, place, preview, build, cancel, replace (equity and option) |
| `chain` | Option chain data (`get`, `expiration`) |
| `option` | Option planning tickets that combine quote, chain, and OCC symbol context |
| `history` | Price history for a symbol (alias: `price-history`) |
| `ta` | Technical analysis with symbol-keyed JSON output (dashboard, sma, ema, rsi, macd, atr, bbands, stoch, adx, vwap, hv, expected-move) |
| `indicators` | Technical analysis dashboard shortcut with symbol-keyed JSON output |
| `analyze` | Combined quote and technical analysis dashboard |
| `market` | Market hours and top movers |
| `symbol` | Build and parse OCC option symbols (no auth required) |
| `instrument` | Search instruments |

### Global flags

```text
--account   Override the default account by hash, account number, or nickname
--verbose   Enable verbose logging
--config    Path to config file (default: ~/.config/schwab-agent/config.json)
--token     Path to token file (default: ~/.config/schwab-agent/token.json)
```

## Safety

schwab-agent has two layers of protection for operations that modify your account:

1. **Config flag**: Set `"i-also-like-to-live-dangerously": true` in your config file to enable mutable operations (order placement, cancellation, replacement).
2. **Preview digest flow**: Add `--save-preview` to `order preview` to store the exact payload locally for 15 minutes, then submit that same payload with `order place --from-preview <digest>`. The digest is bound to the account, operation, endpoint, and canonical order JSON. It is a local safety reference, not a Schwab reservation or idempotency key.

This prevents accidental trades from misconfigured agents.

## Agent integration

schwab-agent is built for coding agents and LLM tools that need a reliable command contract instead of prose-only instructions. Use the generated files in this repo for workflow guidance, but use JSON Schema from the binary as the source of truth for commands and flags.

### Claude Code and other skill-aware tools

Use the checked-in `SKILL.md` when your tool supports Agent Skills, including Claude Code. It contains trigger phrases, command descriptions, flag tables, examples, and MCP server hints generated from the live CLI command tree.

For Claude Code, copy or symlink this repository's generated skill into your skills directory if you want it available outside this repo:

```bash
mkdir -p ~/.claude/skills/schwab-agent
ln -sf "$(pwd)/SKILL.md" ~/.claude/skills/schwab-agent/SKILL.md
```

If you already have a custom skill at that path, review or back it up first because `ln -sf` replaces the existing file or symlink.

You can also keep using the checked-in `SKILL.md` directly when working inside this repository. Regenerate it after command, flag, example, or help text changes:

```bash
make docs
# or
go run ./cmd/generate-docs/
```

`make docs` updates both `SKILL.md` and `llms.txt`. Commit the regenerated files with the code change so agents see the same command surface as the binary.

### OpenCode and Codex

OpenCode and Codex use `AGENTS.md` as project instructions. This repo keeps `AGENTS.md` hand-written because it contains architecture, build, test, safety, and project-convention context that cannot be generated from the CLI tree.

When using OpenCode or Codex, start in the repository root so the tool can load `AGENTS.md`, then point it at `SKILL.md` or `llms.txt` when it needs the full generated command reference. Update `AGENTS.md` by hand when project structure, conventions, build commands, or safety rules change.

### llms.txt

`llms.txt` follows the llms.txt convention for LLM-friendly documentation indexes. Treat it as a compact generated command reference for tools or workflows that prefer a single Markdown index. It is generated by the same `make docs` target as `SKILL.md`.

### Command discovery for shell automation

For shell-based automation, run `schwab-agent --jsonschema=tree` first when you need to discover commands, flags, enum values, defaults, config keys, or environment variable bindings. Treat this JSON Schema tree as the authoritative command contract, then use `SKILL.md`, `llms.txt`, and `--help` for workflow guidance and examples after choosing the command and flags from the schema.

```bash
# Canonical shell-agent discovery path for the full CLI contract
schwab-agent --jsonschema=tree

# Schema for a specific subcommand
schwab-agent quote --jsonschema

# Flat JSON Schema for the entire CLI, useful for tools that do not need hierarchy
schwab-agent --jsonschema
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
  apperr/               Typed error hierarchy with exit codes
  models/               API data structures
  orderbuilder/         Order construction and validation
  output/               JSON success envelopes and structured error output
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
| 10-19 | structcli input errors, such as missing flags, invalid flag values, and unknown commands |
| 20-29 | structcli config or environment errors |

## License

[MIT](LICENSE)

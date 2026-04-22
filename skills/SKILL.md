# schwab-agent

CLI tool for AI agents to trade via Charles Schwab APIs. Single binary, JSON output.

## Install

```bash
go install github.com/major/schwab-agent/cmd/schwab-agent@latest
```

Pre-built binaries for Linux and macOS (amd64/arm64) are on the [releases page](https://github.com/major/schwab-agent/releases).

## Skill Files

| File | Covers |
|------|--------|
| schwab-config-auth.md | Setup, authentication, configuration, environment variables |
| schwab-read.md | Quotes, option chains, price history, accounts, instruments, transactions, movers, market hours, option symbol build/parse |
| schwab-trade.md | Order placement, preview, build, cancel, replace, safety workflow |
| schwab-ta.md | Technical analysis indicators: SMA, EMA, RSI, MACD, ATR, Bollinger Bands, Stochastic, ADX |

## Output Format

All commands return JSON.

- Success: `{"data": ..., "metadata": {...}}`
- Errors: `{"error": {"code": ..., "message": ..., "details": ...}}`
- Partial: `{"data": ..., "errors": [...], "metadata": {...}}`

## Global Flags

| Flag | Description |
|------|-------------|
| `--account` | Override the default account for this command |
| `--verbose` | Enable verbose logging |
| `--config` | Path to config file (default: ~/.config/schwab-agent/config.json) |
| `--token` | Path to token file (default: ~/.config/schwab-agent/token.json) |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Validation or order build error |
| 2 | Symbol or account not found |
| 3 | Auth required, expired, or failed |
| 4 | HTTP error from Schwab API |
| 5 | Order rejected |

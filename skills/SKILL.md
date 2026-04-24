# schwab-agent

CLI tool for AI agents to trade via Charles Schwab APIs. Single binary, JSON output.

## Skill Files

| File | Covers |
|------|--------|
| schwab-config-auth.md | Authentication, configuration, environment variables |
| schwab-read.md | Quotes, option chains, chain expirations, price history, accounts, instruments, transactions, movers, market hours, option symbol build/parse |
| schwab-trade.md | Order listing, placement, preview, cancel, replace, repeat, safety workflow |
| schwab-order-builder.md | Order construction: equity, option, bracket, OCO, vertical, iron condor, straddle, strangle, covered call, collar, calendar, diagonal, FTS |
| schwab-ta.md | Technical analysis: SMA, EMA, RSI, MACD, ATR, Bollinger Bands, Stochastic, ADX, VWAP, HV, expected move |

## Output Format

All commands return JSON envelopes (except `schema`, `order build`, `symbol build/parse` which return raw JSON).

- Success: `{"data": ..., "metadata": {"timestamp": "..."}}`
- Error: `{"error": {"code": "...", "message": "...", "details": "..."}}`
- Partial: `{"data": ..., "errors": [...], "metadata": {...}}`

Error codes: AUTH_REQUIRED, AUTH_EXPIRED, AUTH_CALLBACK, ORDER_REJECTED, SYMBOL_NOT_FOUND, ACCOUNT_NOT_FOUND, HTTP_ERROR, VALIDATION_ERROR, ORDER_BUILD_ERROR

## Global Flags

| Flag | Description |
|------|-------------|
| `--account` | Override the default account for this command |
| `--verbose` | Enable verbose logging |
| `--config` | Path to config file (default: ~/.config/schwab-agent/config.json) |
| `--token` | Path to token file (default: ~/.config/schwab-agent/token.json) |

## Exit Codes

| Code | Meaning | Recovery |
|------|---------|----------|
| 0 | Success | - |
| 1 | Validation or order build error | Fix input flags/values and retry |
| 2 | Symbol or account not found | Verify symbol or run `account numbers` to get valid hashes |
| 3 | Auth required, expired, or failed | Run `auth refresh` or `auth login` |
| 4 | HTTP error from Schwab API | Check `error.details` in response for Schwab's error message |
| 5 | Order rejected | Check `error.details` for rejection reason, adjust order params |

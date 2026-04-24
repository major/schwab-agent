# Market Data and Account Information

Read-only commands. None of these modify your account.

## Option Symbols

Build and parse OCC-format option symbols locally (no API call, no auth required).

```bash
schwab-agent symbol build --underlying AAPL --expiration 2025-06-20 --strike 200 --call
schwab-agent symbol parse "AAPL  250620C00200000"
```

Flags: `--underlying` (req), `--expiration YYYY-MM-DD` (req), `--strike` (req), `--call`/`--put` (one required, mutually exclusive).

Output (both subcommands, raw JSON not envelope-wrapped):

```json
{"symbol": "AAPL  250620C00200000", "underlying": "AAPL", "expiration": "2025-06-20", "put_call": "CALL", "strike": 200}
```

## Quotes

```bash
schwab-agent quote get AAPL
schwab-agent quote get AAPL NVDA TSLA --fields quote,fundamental --indicative
```

| Flag | Description |
|------|-------------|
| `--fields` | Repeatable or comma-separated: quote, fundamental, extended, reference, regular |
| `--indicative` | Request indicative (non-tradeable) quotes |

Multi-symbol requests use partial response if some symbols are missing. Key output fields: last price, bid/ask, volume, 52-week high/low, PE ratio, dividend yield, market cap.

## Option Chains

```bash
schwab-agent chain get AAPL
schwab-agent chain get AAPL --type CALL --strike-count 5 --from-date 2026-05-01 --to-date 2026-05-31
schwab-agent chain get AAPL --strategy ANALYTICAL --volatility 30.5 --underlying-price 148.50 --interest-rate 4.5 --days-to-expiration 45
schwab-agent chain expiration AAPL
```

| Flag | Description |
|------|-------------|
| `--type` | CALL, PUT, or ALL (default) |
| `--strike-count` | Number of strikes above/below ATM |
| `--from-date` / `--to-date` | Filter expirations (YYYY-MM-DD) |
| `--strategy` | SINGLE, ANALYTICAL, COVERED, etc. |
| `--include-underlying-quote` | Include underlying quote data |
| `--interval` | Strike interval for spread strategy chains |
| `--strike` | Filter to specific strike price |
| `--strike-range` | ITM, NTM, OTM, SAK, SBK, SNK, ALL |
| `--volatility` | Volatility for theoretical pricing (with ANALYTICAL) |
| `--underlying-price` | Override underlying price for theoretical calcs |
| `--interest-rate` | Interest rate for theoretical pricing |
| `--days-to-expiration` | DTE for theoretical pricing |

`chain expiration <symbol>` returns available expiration dates without the full chain data.

## Price History

```bash
schwab-agent history get AAPL
schwab-agent history get AAPL --period-type month --period 3 --frequency-type daily --frequency 1
schwab-agent history get AAPL --from 1704067200000 --to 1713657600000
```

| Flag | Description |
|------|-------------|
| `--period-type` | day, month, year, ytd |
| `--period` | Number of periods |
| `--frequency-type` | minute, daily, weekly, monthly |
| `--frequency` | Frequency interval |
| `--from` / `--to` | Start/end time in milliseconds since epoch |

## Accounts

```bash
schwab-agent account list                    # All accounts (enriched with nicknames)
schwab-agent account list --positions        # Include current positions
schwab-agent account get <hash> --positions  # Single account with positions
schwab-agent account numbers                 # Account numbers and hash values
schwab-agent account set-default <hash>      # Set default account
```

Account list and get enrich results with nicknames from the preferences API (best-effort, degrades gracefully).

## Instruments

```bash
schwab-agent instrument search AAPL --projection fundamental
schwab-agent instrument search "Apple" --projection symbol-search
schwab-agent instrument get 037833100    # By CUSIP
```

Projections: symbol-search, symbol-regex, desc-search, desc-regex, search, fundamental.

## Transactions

Account-scoped, under the `account` command. Inherits `--account` flag.

```bash
schwab-agent account transaction list --types TRADE --from 2026-01-01 --to 2026-04-21
schwab-agent account transaction get 123456789
```

Flags: `--types` (TRADE, DIVIDEND, etc.), `--from`, `--to`, `--symbol`.

## Market

```bash
schwab-agent market hours              # All markets
schwab-agent market hours equity       # Specific market (equity, option, bond, future, forex)
schwab-agent market movers '$SPX' --sort PERCENT_CHANGE_UP
schwab-agent market movers EQUITY_ALL --sort VOLUME --frequency 5
```

Movers flags: `--sort` (VOLUME, TRADES, PERCENT_CHANGE_UP, PERCENT_CHANGE_DOWN), `--frequency` (0, 1, 5, 10, 30, 60).

## Recipes

| Intent | Command |
|--------|---------|
| "What's AAPL trading at?" | `quote get AAPL` |
| "Get quotes for my watchlist" | `quote get AAPL NVDA TSLA MSFT` |
| "Fundamental data for AAPL" | `quote get AAPL --fields fundamental` |
| "What options expire next month?" | `chain expiration AAPL` |
| "Show AAPL calls" | `chain get AAPL --type CALL` |
| "Near-the-money AAPL options" | `chain get AAPL --strike-range NTM` |
| "Theoretical pricing for AAPL" | `chain get AAPL --strategy ANALYTICAL --volatility 30 --days-to-expiration 45` |
| "Last 3 months daily prices" | `history get AAPL --period-type month --period 3` |
| "Show my accounts with positions" | `account list --positions` |
| "What accounts do I have?" | `account numbers` |
| "Find a stock by name" | `instrument search "Apple" --projection symbol-search` |
| "Show all trades this year" | `account transaction list --types TRADE --from 2026-01-01` |
| "What are the top movers?" | `market movers $SPX.X` |
| "Is the market open?" | `market hours equity` |
| "Build an option symbol" | `symbol build --underlying AAPL --expiration 2025-06-20 --strike 200 --call` |
| "Parse this OCC symbol" | `symbol parse "AAPL  250620C00200000"` |

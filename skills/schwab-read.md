# Market Data and Account Information

Read-only commands for quotes, option chains, price history, accounts, instruments, transactions, movers, market hours, and option symbol utilities. None of these commands modify your account.

## Option Symbols

Build and parse OCC-format option symbols locally (no API call, no authentication required).

### Build a symbol

```bash
schwab-agent symbol build --underlying AAPL --expiration 2025-06-20 --strike 200 --call
schwab-agent symbol build --underlying SPY --expiration 2025-12-19 --strike 450.50 --put
```

| Flag | Description |
|------|-------------|
| `--underlying` | Underlying stock ticker (required) |
| `--expiration` | Expiration date in YYYY-MM-DD format (required) |
| `--strike` | Strike price (required) |
| `--call` | Call contract |
| `--put` | Put contract |

One of `--call` or `--put` is required (mutually exclusive).

### Parse a symbol

```bash
schwab-agent symbol parse "AAPL  250620C00200000"
```

Takes a single OCC symbol (21 characters, space-padded underlying) and returns its components.

### Output format

Both subcommands return the same JSON shape:

```json
{
  "data": {
    "symbol": "AAPL  250620C00200000",
    "underlying": "AAPL",
    "expiration": "2025-06-20",
    "put_call": "CALL",
    "strike": 200
  },
  "metadata": {"timestamp": "..."}
}
```

## Quotes

```bash
schwab-agent quote get AAPL
schwab-agent quote get AAPL NVDA TSLA MSFT
```

Key output fields: last price, bid/ask, volume, 52-week high/low, PE ratio, dividend yield, market cap.

## Option Chains

```bash
schwab-agent chain get AAPL
schwab-agent chain get AAPL --type CALL
schwab-agent chain get AAPL --type PUT --from-date 2026-05-01 --to-date 2026-05-31
schwab-agent chain get AAPL --strike-count 5
schwab-agent chain get AAPL --strike 150.0 --strike-range NTM
schwab-agent chain get AAPL --strategy ANALYTICAL --volatility 30.5 --underlying-price 148.50 --interest-rate 4.5 --days-to-expiration 45
schwab-agent chain get AAPL --include-underlying-quote
schwab-agent chain expiration AAPL
```

| Flag | Description |
|------|-------------|
| `--type` | CALL, PUT, or ALL (default) |
| `--strike-count` | Number of strikes above/below ATM |
| `--from-date` | Filter expirations from date (YYYY-MM-DD) |
| `--to-date` | Filter expirations to date (YYYY-MM-DD) |
| `--strategy` | SINGLE, ANALYTICAL, COVERED, etc. |
| `--include-underlying-quote` | Include underlying quote data in response |
| `--interval` | Strike interval for spread strategy chains |
| `--strike` | Filter to a specific strike price |
| `--strike-range` | Moneyness filter: ITM, NTM, OTM, SAK, SBK, SNK, ALL |
| `--volatility` | Volatility for theoretical pricing (use with ANALYTICAL strategy) |
| `--underlying-price` | Override underlying price for theoretical calculations |
| `--interest-rate` | Interest rate for theoretical pricing calculations |
| `--days-to-expiration` | Days to expiration for theoretical pricing calculations |

## Price History

```bash
schwab-agent history get AAPL
schwab-agent history get AAPL --period-type month --period 3 --frequency-type daily --frequency 1
schwab-agent history get AAPL --start-date 2026-01-01 --end-date 2026-04-21
```

| Flag | Description |
|------|-------------|
| `--period-type` | day, month, year, ytd |
| `--period` | Number of periods (e.g., 3 for 3 months) |
| `--frequency-type` | minute, daily, weekly, monthly |
| `--frequency` | Frequency interval (e.g., 1 for every day) |
| `--start-date` | Start date (YYYY-MM-DD), use with --end-date |
| `--end-date` | End date (YYYY-MM-DD), use with --start-date |

## Accounts

```bash
schwab-agent account list
schwab-agent account get <hash-value>
schwab-agent account numbers
schwab-agent account set-default <hash-value>
```

Account list and get enrich results with nicknames and primary account flags from the preferences API. This happens automatically and degrades gracefully if the preferences API is unavailable.

## Instruments

```bash
schwab-agent instrument search AAPL
schwab-agent instrument search AAPL --projection fundamental
schwab-agent instrument search "Apple" --projection symbol-search
schwab-agent instrument get 037833100
```

| Projection | Description |
|-----------|-------------|
| `symbol-search` | Search by symbol or name substring |
| `symbol-regex` | Search using regex pattern |
| `desc-search` | Search by description substring |
| `desc-regex` | Search by description regex |
| `fundamental` | Return fundamental data (PE, dividend, etc.) |

## Transactions

Transactions are account-scoped and live under the `account` command. They inherit the `--account` flag from the parent.

```bash
schwab-agent account transaction list
schwab-agent account transaction list --types TRADE
schwab-agent account transaction list --from 2026-01-01 --to 2026-04-21
schwab-agent account transaction get 123456789
```

Flags: `--types` (TRADE/DIVIDEND/etc.), `--from`, `--to`, `--symbol`, `--account` (inherited from parent)

## Market

### Hours

```bash
schwab-agent market hours
schwab-agent market hours equity
```

Available markets: equity, option, bond, future, forex

### Movers

```bash
schwab-agent market movers $SPX.X
schwab-agent market movers $DJI --sort PERCENT_CHANGE_UP
```

Flags: `--sort` (VOLUME/TRADES/PERCENT_CHANGE_UP/PERCENT_CHANGE_DOWN), `--frequency` (0/1/5/10/30/60)

## Recipes

| Intent | Command |
|--------|---------|
| "What's AAPL trading at?" | `schwab-agent quote get AAPL` |
| "Get quotes for my watchlist" | `schwab-agent quote get AAPL NVDA TSLA MSFT` |
| "What options expire next month?" | `schwab-agent chain expiration AAPL` |
| "Show AAPL calls" | `schwab-agent chain get AAPL --type CALL` |
| "Near-the-money AAPL options" | `schwab-agent chain get AAPL --strike-range NTM` |
| "Theoretical pricing for AAPL" | `schwab-agent chain get AAPL --strategy ANALYTICAL --volatility 30 --days-to-expiration 45` |
| "Options at the $150 strike" | `schwab-agent chain get AAPL --strike 150.0` |
| "Last 3 months daily prices" | `schwab-agent history get AAPL --period-type month --period 3` |
| "What accounts do I have?" | `schwab-agent account numbers` |
| "Show my account details" | `schwab-agent account list` |
| "Find a stock by name" | `schwab-agent instrument search "Apple" --projection symbol-search` |
| "Show all trades this year" | `schwab-agent account transaction list --types TRADE --from 2026-01-01` |
| "What are the top movers?" | `schwab-agent market movers $SPX.X` |
| "Is the market open?" | `schwab-agent market hours equity` |
| "Build an option symbol" | `schwab-agent symbol build --underlying AAPL --expiration 2025-06-20 --strike 200 --call` |
| "Parse this OCC symbol" | `schwab-agent symbol parse "AAPL  250620C00200000"` |

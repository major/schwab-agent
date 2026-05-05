---
name: schwab-agent
description: |
  CLI tool for AI agents to trade via Schwab APIs. Use when you need to: manage schwab trading accounts. list accounts with nicknames, view account
  details, look up account numbers and hash values, set a default account, and
  query transaction history. account list and get enrich results with nicknames
  from the preferences api (best-effort, degrades gracefully), get details for a single account by hash value. the hash can be passed as a
  positional argument, via --account, or resolved from the default_account
  config. results are enriched with nicknames from the preferences api. use
  --positions to include current positions, list all linked schwab accounts with nicknames and settings enriched from
  the preferences api. use --positions to include current positions for each
  account in the response. nicknames are loaded best-effort and omitted if
  the preferences api is unavailable. agents that only need account hashes,
  nicknames, and primary account markers should use account summary for a smaller
  one-command accoun...
metadata:
  author: major
  version: dev
---

# schwab-agent

## Instructions

### Available Commands

#### `schwab-agent`

CLI tool for AI agents to trade via Schwab APIs

#### `schwab-agent account`

Manage Schwab trading accounts. List accounts with nicknames, view account
details, look up account numbers and hash values, set a default account, and
query transaction history. Account list and get enrich results with nicknames
from the preferences API (best-effort, degrades gracefully).

#### `schwab-agent account get`

Get details for a single account by hash value. The hash can be passed as a
positional argument, via --account, or resolved from the default_account
config. Results are enriched with nicknames from the preferences API. Use
--positions to include current positions.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--positions` | bool | false | no | Include current positions in the account response |

**Example:**

```bash
schwab-agent account get
  schwab-agent account get ABCDEF1234567890
  schwab-agent account get --positions
```

#### `schwab-agent account list`

List all linked Schwab accounts with nicknames and settings enriched from
the preferences API. Use --positions to include current positions for each
account in the response. Nicknames are loaded best-effort and omitted if
the preferences API is unavailable. Agents that only need account hashes,
nicknames, and primary account markers should use account summary for a smaller
one-command account picker.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--positions` | bool | false | no | Include current positions for each account |

**Example:**

```bash
schwab-agent account list
  schwab-agent account list --positions
```

#### `schwab-agent account numbers`

List account numbers and their corresponding hash values. Hash values are
used as account identifiers in all other commands. Use set-default to store
a hash as the default account.

**Example:**

```bash
schwab-agent account numbers
```

#### `schwab-agent account resolve`

Resolve any account identifier (hash, account number, or nickname) to its
full details including the Schwab hash, account number, nickname, account type,
resolution source, and display label.

The identifier can come from the --account flag, a positional argument, or the
default_account in config.json.

**Example:**

```bash
# Resolve by nickname
  schwab-agent account resolve "My IRA"

  # Resolve by account number
  schwab-agent account resolve 12345678

  # Resolve using the --account flag
  schwab-agent account resolve --account "My IRA"

  # Resolve using the default account from config
  schwab-agent account resolve
```

#### `schwab-agent account set-default`

Set the default account hash in the config file. Commands that require an
account will use this value when --account is not specified. Run account
numbers to find valid hash values.

**Example:**

```bash
schwab-agent account set-default ABCDEF1234567890
```

#### `schwab-agent account summary`

List linked Schwab accounts in a compact, token-efficient shape for agents.
Each entry includes the account hash required by other commands, the readable
account number, and best-effort nickname/primary/type data from user preferences.
Use --positions to include compact holdings with computed cost basis and P&L
in the same response. Use account list when you need full balances or account
list --positions when you need the full Schwab account payload with positions.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--positions` | bool | false | no | Include compact current positions for each account |

**Example:**

```bash
schwab-agent account summary
  schwab-agent account summary --positions
```

#### `schwab-agent account transaction`

Query transaction history for an account. Transactions are account-scoped and
inherit the --account flag from the parent command. Filter by type, date range,
and symbol.

#### `schwab-agent account transaction get`

Get a specific transaction by ID

**Example:**

```bash
schwab-agent account transaction get 98765432101
```

#### `schwab-agent account transaction list`

List transactions for an account with optional filtering by type, date range,
and symbol. Date values use ISO 8601 format (YYYY-MM-DDTHH:MM:SSZ). Uses the
default account unless --account is specified.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--from` | string | - | no | Start date (YYYY-MM-DDTHH:MM:SSZ) |
| `--symbol` | string | - | no | Filter by symbol |
| `--to` | string | - | no | End date (YYYY-MM-DDTHH:MM:SSZ) |
| `--types` | string | - | no | Transaction type filter (TRADE, DIVIDEND, etc.) |

**Example:**

```bash
schwab-agent account transaction list
  schwab-agent account transaction list --types TRADE --symbol AAPL
  schwab-agent account transaction list --from 2025-01-01T00:00:00Z --to 2025-01-31T23:59:59Z
```

#### `schwab-agent analyze`

Fetch a quote and compute an opinionated TA dashboard for each symbol in a
single CLI call. Combines "quote get" and "ta dashboard" output into per-symbol
nesting so agents get both market data and technical context without two
round-trips. Partial failures are reported per-symbol: if the quote succeeds
but TA fails, that symbol appears with its quote and a null analysis field.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--points` | int | 1 | no | Number of TA output points (default 1; 0 = all) |

**Example:**

```bash
schwab-agent analyze AAPL
  schwab-agent analyze AAPL NVDA
  schwab-agent analyze AAPL --interval weekly --points 5
```

#### `schwab-agent auth`

Manage OAuth2 authentication with the Schwab API. Login starts the OAuth flow,
status checks token expiration, and refresh forces a token refresh. Auth
commands bypass the global auth check so they can run without existing tokens.
Tokens are stored locally and refreshed automatically during normal use.

#### `schwab-agent auth login`

Start the OAuth2 login flow to authenticate with Schwab. Opens a browser by
default; use --no-browser to print the authorization URL instead. After
authorization, tokens are stored locally. If exactly one account is found,
it is automatically set as the default.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--no-browser` | bool | false | no | Print the authorization URL in the JSON response instead of opening a browser |

**Example:**

```bash
schwab-agent auth login
  schwab-agent auth login --no-browser
```

#### `schwab-agent auth refresh`

Force a refresh of the current access token without re-authenticating. Requires
a valid refresh token (not expired). If the refresh token is stale (older than
6.5 days), run auth login again instead.

**Example:**

```bash
schwab-agent auth refresh
```

#### `schwab-agent auth status`

Show current authentication status including access token expiration, refresh
token expiration, configured default account, and client ID (redacted). Use
this to check whether tokens are still valid before running other commands.

**Example:**

```bash
schwab-agent auth status
```

#### `schwab-agent chain`

Option chain operations

#### `schwab-agent chain expiration`

Get available option expiration dates for a symbol without fetching the full
chain data. Useful for discovering valid expirations before requesting a
detailed chain.

**Example:**

```bash
schwab-agent chain expiration AAPL
```

#### `schwab-agent chain get`

Get the full option chain for a symbol with optional filtering by contract type,
strike range, expiration dates, and strategy. Use --strategy ANALYTICAL with
--volatility and --days-to-expiration for theoretical pricing. Filter to
specific moneyness with --strike-range (ITM, NTM, OTM, ALL).

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--days-to-expiration` | string | - | no | Days to expiration for theoretical pricing calculations |
| `--from-date` | string | - | no | Start date (YYYY-MM-DD) |
| `--include-underlying-quote` | bool | false | no | Include underlying quote data in response |
| `--interest-rate` | string | - | no | Interest rate for theoretical pricing calculations |
| `--interval` | string | - | no | Strike interval for spread strategy chains |
| `--strategy` | string | - | no | Option pricing strategy {,ANALYTICAL,BUTTERFLY,CALENDAR,COLLAR,CONDOR,COVERED,DIAGONAL,ROLL,SINGLE,STRADDLE,STRANGLE,VERTICAL} |
| `--strike` | string | - | no | Filter to a specific strike price |
| `--strike-count` | string | - | no | Number of strikes to return |
| `--strike-range` | string | - | no | Moneyness filter: ITM, NTM, OTM, SAK, SBK, SNK, or ALL {,ALL,ITM,NTM,OTM,SAK,SBK,SNK} |
| `--to-date` | string | - | no | End date (YYYY-MM-DD) |
| `--type` | string | - | no | Contract type: CALL, PUT, or ALL {,ALL,CALL,PUT} |
| `--underlying-price` | string | - | no | Override underlying price for theoretical calculations |
| `--volatility` | string | - | no | Volatility for theoretical pricing calculations |

**Example:**

```bash
schwab-agent chain get AAPL
  schwab-agent chain get AAPL --type CALL --strike-count 5
  schwab-agent chain get AAPL --from-date 2025-06-01 --to-date 2025-07-31 --type PUT
  schwab-agent chain get AAPL --strategy ANALYTICAL --volatility 30.5 --days-to-expiration 45
  schwab-agent chain get AAPL --strike-range NTM --include-underlying-quote
```

#### `schwab-agent completion`

Generate shell completion scripts for bash, zsh, fish, or powershell.

#### `schwab-agent completion bash`

Generate bash completion script.

To load completions in your current shell session:

	source <(schwab-agent completion bash)

To load completions for every new session, execute once:

	schwab-agent completion bash | sudo tee /etc/bash_completion.d/schwab-agent

or

	schwab-agent completion bash >> ~/.bashrc


#### `schwab-agent completion fish`

Generate fish completion script.

To load completions in your current shell session:

	schwab-agent completion fish | source

To load completions for every new session, execute once:

	schwab-agent completion fish > ~/.config/fish/completions/schwab-agent.fish


#### `schwab-agent completion powershell`

Generate powershell completion script.

To load completions in your current shell session:

	schwab-agent completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


#### `schwab-agent completion zsh`

Generate zsh completion script.

To load completions in your current shell session:

	source <(schwab-agent completion zsh)

To load completions for every new session, execute once:

	schwab-agent completion zsh > "${fpath[1]}/_schwab-agent"


#### `schwab-agent config-keys`

Show config keys derived from the current Cobra command tree and their corresponding flags.

#### `schwab-agent env-vars`

Show environment variables that schwab-agent reads for authentication, API configuration, state, and debug behavior.

#### `schwab-agent history`

Retrieve price history for a symbol

#### `schwab-agent history get`

Get price history candles for a symbol. Supports configurable period types (day,
month, year, ytd), frequency types (minute, daily, weekly, monthly), and
date ranges via epoch milliseconds. Returns OHLCV candle data.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--frequency` | string | - | no | Frequency value {,1,10,15,30,5} |
| `--frequency-type` | string | - | no | Frequency type (minute, daily, weekly, monthly) {,daily,minute,monthly,weekly} |
| `--from` | string | - | no | Start date (milliseconds since epoch) |
| `--period` | string | - | no | Number of periods |
| `--period-type` | string | - | no | Period type (day, month, year, ytd) {,day,month,year,ytd} |
| `--to` | string | - | no | End date (milliseconds since epoch) |

**Example:**

```bash
schwab-agent history get AAPL
  schwab-agent history get AAPL --period-type month --period 3 --frequency-type daily --frequency 1
  schwab-agent history get AAPL --period-type day --period 5 --frequency-type minute --frequency 15
  schwab-agent history get AAPL --from 1735689600000 --to 1743379200000
```

#### `schwab-agent indicators`

Shortcut for "ta dashboard". Computes an opinionated technical-analysis
dashboard for one or more symbols with one price-history fetch per symbol.
Includes SMA 21/50/200, RSI 14, MACD 12/26/9, ATR 14, Bollinger Bands 20/2,
volume context, and 20/252-candle high-low ranges.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |

**Example:**

```bash
schwab-agent indicators AAPL
  schwab-agent indicators AAPL --points 5
  schwab-agent indicators NVDA --interval weekly --points 10
  schwab-agent indicators AAPL MSFT NVDA
```

#### `schwab-agent instrument`

Search and look up instruments

#### `schwab-agent instrument get`

Get instrument details by CUSIP identifier. Returns the instrument type,
description, exchange, and other metadata for the specified CUSIP.

**Example:**

```bash
schwab-agent instrument get 037833100
```

#### `schwab-agent instrument search`

Search for instruments by symbol or description. Use --projection to control
the search type: symbol-search (default), symbol-regex, desc-search,
desc-regex, search, or fundamental.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--projection` | string | symbol-search | no | Search projection (symbol-search, symbol-regex, desc-search, desc-regex, search, fundamental) |

**Example:**

```bash
schwab-agent instrument search AAPL
  schwab-agent instrument search "Apple" --projection desc-search
  schwab-agent instrument search AAPL --projection fundamental
```

#### `schwab-agent market`

Market hours and top movers

#### `schwab-agent market hours`

Get current market hours for one or more markets. With no arguments, returns
hours for all markets (equity, option, bond, future, forex). Specify one or
more market names to filter.

**Example:**

```bash
schwab-agent market hours
  schwab-agent market hours equity
  schwab-agent market hours equity option
```

#### `schwab-agent market movers`

Get top movers for a market index. Sort by VOLUME, TRADES, PERCENT_CHANGE_UP,
or PERCENT_CHANGE_DOWN. Use --frequency to filter by minimum percent change
magnitude (0, 1, 5, 10, 30, 60). Quote shell-special characters in index
names (e.g. '$SPX').

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--frequency` | string | - | no | Minimum percent change magnitude (0, 1, 5, 10, 30, 60) {,0,1,10,30,5,60} |
| `--sort` | string | - | no | Sort order (VOLUME, TRADES, PERCENT_CHANGE_UP, PERCENT_CHANGE_DOWN) {,PERCENT_CHANGE_DOWN,PERCENT_CHANGE_UP,TRADES,VOLUME} |

**Example:**

```bash
schwab-agent market movers '$SPX' --sort PERCENT_CHANGE_UP
  schwab-agent market movers '$DJI' --sort VOLUME --frequency 5
  schwab-agent market movers EQUITY_ALL --sort PERCENT_CHANGE_UP
```

#### `schwab-agent option`

Option planning workflows that combine quote, chain, and symbol context for agents.

#### `schwab-agent option ticket`

Build an option planning ticket from live market data. The ticket combines
the underlying quote, a narrowly filtered option chain, the matching contract,
and the OCC symbol in one read-only command. Use order preview option when you
are ready to turn the ticket into a Schwab preview request.

#### `schwab-agent option ticket get`

Get a compact option ticket for one underlying, expiration, strike, and
contract side. This collapses the common agent workflow of quote lookup, chain
lookup, and OCC symbol construction into one read-only CLI call.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call` | bool | false | no | Select the call contract |
| `--expiration` | string | - | no | Option expiration date (YYYY-MM-DD) |
| `--put` | bool | false | no | Select the put contract |
| `--strike` | float64 | 0 | no | Option strike price |

**Example:**

```bash
schwab-agent option ticket get AAPL --expiration 2026-01-16 --strike 200 --call
  schwab-agent option ticket get TSLA --expiration 2026-03-20 --strike 180 --put
```

#### `schwab-agent order`

Manage orders across your Schwab accounts. Supports listing, viewing, placing,
previewing, building, canceling, and replacing orders. Placing, canceling, and
replacing orders require the "i-also-like-to-live-dangerously" config flag.
Duration aliases GTC, FOK, and IOC are accepted.

#### `schwab-agent order build`

Build order request JSON locally without calling the API or requiring
authentication. Output is raw JSON (not envelope-wrapped) that can be piped to
order preview or order place --spec - for a staged workflow. Supports equity,
option, bracket, OCO, and multi-leg option strategies.

#### `schwab-agent order build back-ratio`

Build an option back-ratio spread order request JSON. Quantity controls the
short leg, and --long-ratio controls how many long contracts are bought per
short contract. Use --debit or --credit because back-ratios can price either way.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call` | bool | false | no | Call back-ratio |
| `--close` | bool | false | no | Closing position |
| `--credit` | bool | false | no | Build as NET_CREDIT |
| `--debit` | bool | false | no | Build as NET_DEBIT |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--long-ratio` | float64 | 2 | no | Long contracts per short contract |
| `--long-strike` | float64 | 0 | yes | Long option strike |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net debit or credit amount |
| `--put` | bool | false | no | Put back-ratio |
| `--quantity` | float64 | 0 | yes | Short-leg contract quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--short-strike` | float64 | 0 | yes | Short option strike |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build back-ratio --underlying F --expiration 2026-06-18 --short-strike 12 --long-strike 14 --call --open --quantity 1 --long-ratio 2 --debit --price 0.20
```

#### `schwab-agent order build bracket`

Build a bracket order request JSON with entry and automatic exit legs. At
least one of --take-profit or --stop-loss is required. Exit instructions are
auto-inverted from the entry action.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--price` | float64 | 0 | no | Entry price |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--stop-loss` | float64 | 0 | no | Stop-loss exit price |
| `--symbol` | string | - | yes | Equity symbol |
| `--take-profit` | float64 | 0 | no | Take-profit exit price |
| `--type` | string | - | no | Entry order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |

**Example:**

```bash
schwab-agent order build bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120
  schwab-agent order build bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 180 --stop-loss 170
```

#### `schwab-agent order build butterfly`

Build a butterfly spread order request JSON. Uses three ordered strikes at
the same expiration: lower wing, middle body, upper wing. Buying opens a long
butterfly with long wings and two short body contracts.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--buy` | bool | false | no | Buy a long butterfly |
| `--call` | bool | false | no | Call butterfly |
| `--close` | bool | false | no | Closing position |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--lower-strike` | float64 | 0 | yes | Lower wing strike |
| `--middle-strike` | float64 | 0 | yes | Middle body strike |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net debit or credit amount |
| `--put` | bool | false | no | Put butterfly |
| `--quantity` | float64 | 0 | yes | Wing contract quantity |
| `--sell` | bool | false | no | Sell a short butterfly |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--underlying` | string | - | yes | Underlying symbol |
| `--upper-strike` | float64 | 0 | yes | Upper wing strike |

**Example:**

```bash
schwab-agent order build butterfly --underlying F --expiration 2026-06-18 --lower-strike 10 --middle-strike 12 --upper-strike 14 --call --buy --open --quantity 1 --price 0.50
```

#### `schwab-agent order build buy-with-stop`

Build a buy-with-stop bracket order request JSON locally with automatic stop-loss protection.
The entry fills first as a TRIGGER parent, then the stop-loss activates automatically.
Optional --take-profit adds a second exit leg, creating an OCO structure where one exit fill cancels the other.
Exit legs are always GOOD_TILL_CANCEL regardless of --duration, which only controls the entry order.
IMPORTANT: Bracket orders cannot be modified via order replace. To adjust, cancel the entire order and re-place with new parameters.
MARKET entry (--type MARKET) skips price validation for stop-loss placement because there is no fixed entry price.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--duration` | string | - | no | Entry duration (exit legs are always GTC) {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--price` | float64 | 0 | no | Entry limit price (required for LIMIT orders, omit for MARKET) |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--stop-loss` | float64 | 0 | yes | Stop trigger price - becomes market sell when hit |
| `--symbol` | string | - | yes | Stock symbol (e.g., AAPL) |
| `--take-profit` | float64 | 0 | no | Optional take-profit limit price |
| `--type` | string | - | no | Entry order type (LIMIT or MARKET, default LIMIT) {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |

**Example:**

```bash
# Buy 10 shares of AAPL at $150 with stop-loss at $140
  schwab-agent order build buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140

  # Market entry with stop-loss (no price validation)
  schwab-agent order build buy-with-stop --symbol AAPL --quantity 10 --type MARKET --stop-loss 140

  # With take-profit (creates OCO exit structure)
  schwab-agent order build buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140 --take-profit 170
```

#### `schwab-agent order build calendar`

Build a calendar spread order request JSON. Same strike price across two
different expirations: sells the near-term contract and buys the far-term
contract. Profits from time decay differential between the two legs.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call` | bool | false | no | Call calendar spread |
| `--close` | bool | false | no | Closing position |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--far-expiration` | string | - | yes | Far-term expiration date (YYYY-MM-DD) |
| `--near-expiration` | string | - | yes | Near-term expiration date (YYYY-MM-DD) |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net debit amount |
| `--put` | bool | false | no | Put calendar spread |
| `--quantity` | float64 | 0 | yes | Number of contracts |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--strike` | float64 | 0 | yes | Strike price (shared by both legs) |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build calendar --underlying F --near-expiration 2026-05-16 --far-expiration 2026-07-17 --strike 12 --call --open --quantity 1 --price 0.50
```

#### `schwab-agent order build collar`

Build a collar-with-stock order request JSON. Combines long stock, a
protective long put, and a covered short call. Limits downside risk while
capping upside. Quantity 1 means 100 shares plus 1 put and 1 call contract.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call-strike` | float64 | 0 | yes | Covered call strike price |
| `--close` | bool | false | no | Closing position |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date for both options (YYYY-MM-DD) |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net debit amount |
| `--put-strike` | float64 | 0 | yes | Protective put strike price |
| `--quantity` | float64 | 0 | yes | Number of contracts (1 contract = 100 shares) |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build collar --underlying F --put-strike 10 --call-strike 14 --expiration 2026-06-18 --quantity 1 --open --price 12.00
```

#### `schwab-agent order build condor`

Build a same-expiration condor spread order request JSON. Uses four ordered
strikes: lower wing, lower-middle short, upper-middle short, upper wing. Buying
opens a long condor; selling opens the inverse short condor.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--buy` | bool | false | no | Buy a long condor |
| `--call` | bool | false | no | Call condor |
| `--close` | bool | false | no | Closing position |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--lower-middle-strike` | float64 | 0 | yes | Lower middle strike |
| `--lower-strike` | float64 | 0 | yes | Lower wing strike |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net debit or credit amount |
| `--put` | bool | false | no | Put condor |
| `--quantity` | float64 | 0 | yes | Contract quantity per strike |
| `--sell` | bool | false | no | Sell a short condor |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--underlying` | string | - | yes | Underlying symbol |
| `--upper-middle-strike` | float64 | 0 | yes | Upper middle strike |
| `--upper-strike` | float64 | 0 | yes | Upper wing strike |

**Example:**

```bash
schwab-agent order build condor --underlying F --expiration 2026-06-18 --lower-strike 10 --lower-middle-strike 12 --upper-middle-strike 14 --upper-strike 16 --call --buy --open --quantity 1 --price 0.75
```

#### `schwab-agent order build covered-call`

Build a covered call order request JSON that atomically buys shares and sells
a call. Quantity 1 means 100 shares plus 1 call contract. The --price is the
net debit per share after call premium. To sell calls against shares you
already own, use order place option --action SELL_TO_OPEN instead.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--price` | float64 | 0 | yes | Net debit amount |
| `--quantity` | float64 | 0 | yes | Number of contracts (1 contract = 100 shares) |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--strike` | float64 | 0 | yes | Call strike price |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build covered-call --underlying F --expiration 2026-06-18 --strike 14 --quantity 1 --price 12.00
```

#### `schwab-agent order build diagonal`

Build a diagonal spread order request JSON. Different strikes AND different
expirations: sells the near-term contract at one strike and buys the far-term
contract at a different strike. Combines elements of vertical and calendar spreads.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call` | bool | false | no | Call diagonal spread |
| `--close` | bool | false | no | Closing position |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--far-expiration` | string | - | yes | Far-term expiration date (YYYY-MM-DD) |
| `--far-strike` | float64 | 0 | yes | Strike price for the far-term (bought) leg |
| `--near-expiration` | string | - | yes | Near-term expiration date (YYYY-MM-DD) |
| `--near-strike` | float64 | 0 | yes | Strike price for the near-term (sold) leg |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net debit amount |
| `--put` | bool | false | no | Put diagonal spread |
| `--quantity` | float64 | 0 | yes | Number of contracts |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build diagonal --underlying F --near-strike 12 --far-strike 14 --near-expiration 2026-05-16 --far-expiration 2026-07-17 --call --open --quantity 1 --price 0.50
```

#### `schwab-agent order build double-diagonal`

Build a double diagonal spread order request JSON with far long put/call legs
and near short put/call legs. The near expiration must come before the far
expiration, and strikes must keep the put side below the call side.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call-far-strike` | float64 | 0 | yes | Far call long strike |
| `--call-near-strike` | float64 | 0 | yes | Near call short strike |
| `--close` | bool | false | no | Closing position |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--far-expiration` | string | - | yes | Far expiration for long legs (YYYY-MM-DD) |
| `--near-expiration` | string | - | yes | Near expiration for short legs (YYYY-MM-DD) |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net debit or credit amount |
| `--put-far-strike` | float64 | 0 | yes | Far put long strike |
| `--put-near-strike` | float64 | 0 | yes | Near put short strike |
| `--quantity` | float64 | 0 | yes | Contract quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build double-diagonal --underlying F --near-expiration 2026-06-18 --far-expiration 2026-07-17 --put-far-strike 9 --put-near-strike 10 --call-near-strike 14 --call-far-strike 15 --open --quantity 1 --price 0.80
```

#### `schwab-agent order build equity`

Build an equity order request JSON. Validates flags and outputs the order
payload without placing it. Pipe to order place --spec - to execute,
or to order preview --spec - to check estimated commissions.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--activation-price` | float64 | 0 | no | Price that activates the trailing stop |
| `--destination` | string | - | no | Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO) {,AMEX,AUTO,BATS,BOX,C2,CBOE,ECN_ARCA,INET,ISE,NASDAQ,NYSE,PHLX} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--price` | float64 | 0 | no | Limit price |
| `--price-link-basis` | string | - | no | Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--price-link-type` | string | - | no | Price link offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--special-instruction` | string | - | no | Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE) {,ALL_OR_NONE,ALL_OR_NONE_DO_NOT_REDUCE,DO_NOT_REDUCE} |
| `--stop-link-basis` | string | - | no | Trailing stop reference price (LAST, BID, ASK, MARK) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--stop-link-type` | string | - | no | Trailing stop offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--stop-offset` | float64 | 0 | no | Trailing stop offset amount |
| `--stop-price` | float64 | 0 | no | Stop price |
| `--stop-type` | string | - | no | Trailing stop trigger type (STANDARD, BID, ASK, LAST, MARK) {,ASK,BID,LAST,MARK,STANDARD} |
| `--symbol` | string | - | yes | Equity symbol |
| `--type` | string | - | no | Order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |

**Example:**

```bash
schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --duration DAY
  schwab-agent order build equity --symbol AAPL --action SELL --quantity 10 --type STOP --stop-price 145
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order place --spec -
```

#### `schwab-agent order build fts`

Build a first-triggers-second order from two JSON order specs. The primary
order executes first; when it fills, the secondary order is automatically
triggered. Both --primary and --secondary accept inline JSON, @file, or
- for stdin.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--primary` | string | - | yes | Primary order spec (inline JSON, @file, or - for stdin) |
| `--secondary` | string | - | yes | Secondary order spec (inline JSON, @file, or - for stdin) |

**Example:**

```bash
schwab-agent order build fts --primary @entry.json --secondary @exit.json
  schwab-agent order build fts --primary '{"orderType":"LIMIT",...}' --secondary '{"orderType":"STOP",...}'
```

#### `schwab-agent order build iron-condor`

Build an iron condor order request JSON. Four strikes must be ordered:
put-long < put-short < call-short < call-long. Opening an iron condor
generates a NET_CREDIT.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call-long-strike` | float64 | 0 | yes | Highest strike: call being bought (protection) |
| `--call-short-strike` | float64 | 0 | yes | Call being sold (premium) |
| `--close` | bool | false | no | Closing position |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net credit or debit amount |
| `--put-long-strike` | float64 | 0 | yes | Lowest strike: put being bought (protection) |
| `--put-short-strike` | float64 | 0 | yes | Put being sold (premium) |
| `--quantity` | float64 | 0 | yes | Number of contracts |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build iron-condor --underlying F --expiration 2026-06-18 --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 --open --quantity 1 --price 0.50
```

#### `schwab-agent order build oco`

Build a one-cancels-other order request JSON for an existing position. When
one exit fills, the other is canceled. At least one of --take-profit or
--stop-loss is required.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Exit action (SELL to close long, BUY to close short) {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--stop-loss` | float64 | 0 | no | Stop-loss exit price (stop order) |
| `--symbol` | string | - | yes | Equity symbol |
| `--take-profit` | float64 | 0 | no | Take-profit exit price (limit order) |

**Example:**

```bash
schwab-agent order build oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140
  schwab-agent order build oco --symbol TSLA --action BUY --quantity 10 --stop-loss 250
```

#### `schwab-agent order build option`

Build a single-leg option order request JSON. Requires --underlying,
--expiration, --strike, and exactly one of --call or --put. Output is raw JSON
suitable for piping to order place or order preview.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--call` | bool | false | no | Call option |
| `--destination` | string | - | no | Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO) {,AMEX,AUTO,BATS,BOX,C2,CBOE,ECN_ARCA,INET,ISE,NASDAQ,NYSE,PHLX} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--price` | float64 | 0 | no | Limit price |
| `--price-link-basis` | string | - | no | Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--price-link-type` | string | - | no | Price link offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--put` | bool | false | no | Put option |
| `--quantity` | float64 | 0 | yes | Contract quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--special-instruction` | string | - | no | Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE) {,ALL_OR_NONE,ALL_OR_NONE_DO_NOT_REDUCE,DO_NOT_REDUCE} |
| `--strike` | float64 | 0 | yes | Strike price |
| `--type` | string | - | no | Order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00
  schwab-agent order build option --underlying AAPL --expiration 2025-06-20 --strike 190 --put --action SELL_TO_OPEN --quantity 1
```

#### `schwab-agent order build straddle`

Build a straddle order request JSON. A straddle buys (or sells) a call and
put at the same strike price and expiration. Use --buy for long straddles
(expecting large price movement) or --sell for short straddles.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--buy` | bool | false | no | Buy the straddle (long, net debit) |
| `--close` | bool | false | no | Closing position |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net debit or credit amount |
| `--quantity` | float64 | 0 | yes | Number of contracts |
| `--sell` | bool | false | no | Sell the straddle (short, net credit) |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--strike` | float64 | 0 | yes | Strike price (shared by call and put legs) |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build straddle --underlying F --expiration 2026-06-18 --strike 12 --buy --open --quantity 1 --price 1.50
  schwab-agent order build straddle --underlying F --expiration 2026-06-18 --strike 12 --sell --open --quantity 1 --price 1.50
```

#### `schwab-agent order build strangle`

Build a strangle order request JSON. A strangle uses different strikes for
the call and put legs. Use --buy for long strangles (expecting volatility)
or --sell for short strangles (expecting low volatility).

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--buy` | bool | false | no | Buy the strangle (long, net debit) |
| `--call-strike` | float64 | 0 | yes | Strike price for the call leg |
| `--close` | bool | false | no | Closing position |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net debit or credit amount |
| `--put-strike` | float64 | 0 | yes | Strike price for the put leg |
| `--quantity` | float64 | 0 | yes | Number of contracts |
| `--sell` | bool | false | no | Sell the strangle (short, net credit) |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build strangle --underlying F --expiration 2026-06-18 --call-strike 14 --put-strike 10 --buy --open --quantity 1 --price 0.50
  schwab-agent order build strangle --underlying F --expiration 2026-06-18 --call-strike 14 --put-strike 10 --sell --open --quantity 1 --price 0.50
```

#### `schwab-agent order build vertical`

Build a vertical spread (bull/bear call or put spread) order request JSON.
Use --long-strike for the strike bought and --short-strike for the strike sold.
NET_DEBIT or NET_CREDIT is auto-determined from the strike relationship.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call` | bool | false | no | Call spread |
| `--close` | bool | false | no | Closing position |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--long-strike` | float64 | 0 | yes | Strike price of the option being bought |
| `--open` | bool | false | no | Opening position |
| `--price` | float64 | 0 | yes | Net debit or credit amount |
| `--put` | bool | false | no | Put spread |
| `--quantity` | float64 | 0 | yes | Number of contracts |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--short-strike` | float64 | 0 | yes | Strike price of the option being sold |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
# Bull call spread
  schwab-agent order build vertical --underlying F --expiration 2026-06-18 --long-strike 12 --short-strike 14 --call --open --quantity 1 --price 0.50
  # Bear put spread
  schwab-agent order build vertical --underlying F --expiration 2026-06-18 --long-strike 14 --short-strike 12 --put --open --quantity 1 --price 0.50
```

#### `schwab-agent order build vertical-roll`

Build a vertical roll order request JSON that closes one vertical spread and
opens another in a single order. Use --debit or --credit to state the roll's net
pricing direction.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call` | bool | false | no | Call vertical roll |
| `--close-expiration` | string | - | yes | Expiration of the vertical being closed (YYYY-MM-DD) |
| `--close-long-strike` | float64 | 0 | yes | Long strike of the vertical being closed |
| `--close-short-strike` | float64 | 0 | yes | Short strike of the vertical being closed |
| `--credit` | bool | false | no | Build as NET_CREDIT |
| `--debit` | bool | false | no | Build as NET_DEBIT |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--open-expiration` | string | - | yes | Expiration of the vertical being opened (YYYY-MM-DD) |
| `--open-long-strike` | float64 | 0 | yes | Long strike of the vertical being opened |
| `--open-short-strike` | float64 | 0 | yes | Short strike of the vertical being opened |
| `--price` | float64 | 0 | yes | Net debit or credit amount |
| `--put` | bool | false | no | Put vertical roll |
| `--quantity` | float64 | 0 | yes | Contract quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order build vertical-roll --underlying F --close-expiration 2026-06-18 --open-expiration 2026-07-17 --close-long-strike 12 --close-short-strike 14 --open-long-strike 13 --open-short-strike 15 --call --debit --quantity 1 --price 0.25
```

#### `schwab-agent order cancel`

Cancel an existing order by ID. Requires the "i-also-like-to-live-dangerously"
config flag. The order ID can be passed as a positional argument or with
--order-id (flag takes priority).

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--order-id` | string | - | no | Order ID |

**Example:**

```bash
schwab-agent order cancel 1234567890
   schwab-agent order cancel --order-id 1234567890
```

#### `schwab-agent order get`

Retrieve a single order by its ID. The order ID can be passed as a positional
argument or with the --order-id flag. When both are provided, the flag takes
priority. Requires a default account or --account flag.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--order-id` | string | - | no | Order ID |

**Example:**

```bash
schwab-agent order get 1234567890
  schwab-agent order get --order-id 1234567890
```

#### `schwab-agent order list`

List orders for the current account, or all accounts when no --account is set.
By default, terminal statuses (FILLED, CANCELED, REJECTED, EXPIRED, REPLACED)
are filtered out to show only actionable orders. Use --status all to see
everything. Use --recent for an agent-friendly recent activity view that keeps
terminal statuses and narrows the default lookback to 24 hours. Multiple
--status values fan out into separate API calls with merged, deduplicated
results.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--from` | string | - | no | Filter by entered time lower bound |
| `--recent` | bool | false | no | Show recent order activity, including terminal statuses, from the last 24 hours unless --from is set |
| `--status` | stringSlice | [] | no | Filter by order status (repeatable, use 'all' for unfiltered): WORKING, PENDING_ACTIVATION, FILLED, EXPIRED, CANCELED, REJECTED, etc. |
| `--to` | string | - | no | Filter by entered time upper bound |

**Example:**

```bash
schwab-agent order list
  schwab-agent order list --recent
  schwab-agent order list --status all
  schwab-agent order list --status WORKING --status PENDING_ACTIVATION
  schwab-agent order list --status WORKING,FILLED,EXPIRED
  schwab-agent order list --from 2025-01-01 --to 2025-01-31
```

#### `schwab-agent order place`

Place an order via subcommand (equity, option, bracket, oco), from a JSON spec
with --spec, or from an exact saved preview with --from-preview. Requires
"i-also-like-to-live-dangerously" set to true in config.json. The safest workflow
is to run order preview --save-preview, inspect the response, then place with the
returned previewDigest.digest value.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--from-preview` | string | - | no | Place the exact order payload saved by order preview --save-preview |
| `--spec` | string | - | no | Inline JSON, @file, or - for stdin |

**Example:**

```bash
# Place from a JSON file
  schwab-agent order place --spec @order.json
  # Place the exact payload saved by a previous preview
  schwab-agent order preview equity --account abc123 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --save-preview
  schwab-agent order place --from-preview <digest>
  # Place from stdin (piped from order build)
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order place --spec -
  # Place from inline JSON
  schwab-agent order place --spec '{"orderType":"LIMIT",...}'
```

#### `schwab-agent order place bracket`

Place a bracket order that combines an entry trade with automatic exit orders.
At least one of --take-profit or --stop-loss is required. Exit instructions are
auto-inverted from the entry action (BUY entry creates SELL exits). Canceling
the parent cascades to all child orders.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--price` | float64 | 0 | no | Entry price |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--stop-loss` | float64 | 0 | no | Stop-loss exit price |
| `--symbol` | string | - | yes | Equity symbol |
| `--take-profit` | float64 | 0 | no | Take-profit exit price |
| `--type` | string | - | no | Entry order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |

**Example:**

```bash
# Buy with both take-profit and stop-loss
  schwab-agent order place bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120
  # Buy with only a stop-loss safety net
  schwab-agent order place bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 180 --stop-loss 170
  # Buy with only a take-profit target
  schwab-agent order place bracket --symbol TSLA --action BUY --quantity 5 --type MARKET --take-profit 300
```

#### `schwab-agent order place buy-with-stop`

Place a buy-with-stop bracket order through the Schwab API with automatic stop-loss protection.
The entry fills first as a TRIGGER parent, then the stop-loss activates automatically.
Optional --take-profit adds a second exit leg, creating an OCO structure where one exit fill cancels the other.
Exit legs are always GOOD_TILL_CANCEL regardless of --duration, which only controls the entry order.
IMPORTANT: Bracket orders cannot be modified via order replace. To adjust, cancel the entire order and re-place with new parameters.
MARKET entry (--type MARKET) skips price validation for stop-loss placement because there is no fixed entry price.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--duration` | string | - | no | Entry duration (exit legs are always GTC) {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--price` | float64 | 0 | no | Entry limit price (required for LIMIT orders, omit for MARKET) |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--stop-loss` | float64 | 0 | yes | Stop trigger price - becomes market sell when hit |
| `--symbol` | string | - | yes | Stock symbol (e.g., AAPL) |
| `--take-profit` | float64 | 0 | no | Optional take-profit limit price |
| `--type` | string | - | no | Entry order type (LIMIT or MARKET, default LIMIT) {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |

**Example:**

```bash
# Buy 10 shares of AAPL at $150 with stop-loss at $140
  schwab-agent order place buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140

  # Market entry with stop-loss (no price validation)
  schwab-agent order place buy-with-stop --symbol AAPL --quantity 10 --type MARKET --stop-loss 140

  # With take-profit (creates OCO exit structure)
  schwab-agent order place buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140 --take-profit 170
```

#### `schwab-agent order place equity`

Place an equity (stock) order. Supports MARKET, LIMIT, STOP, STOP_LIMIT, and
TRAILING_STOP order types. Default type is MARKET if --type is omitted. Duration
aliases GTC, FOK, and IOC are accepted alongside their full names. Requires
i-also-like-to-live-dangerously in config for placement.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--activation-price` | float64 | 0 | no | Price that activates the trailing stop |
| `--destination` | string | - | no | Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO) {,AMEX,AUTO,BATS,BOX,C2,CBOE,ECN_ARCA,INET,ISE,NASDAQ,NYSE,PHLX} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--price` | float64 | 0 | no | Limit price |
| `--price-link-basis` | string | - | no | Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--price-link-type` | string | - | no | Price link offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--special-instruction` | string | - | no | Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE) {,ALL_OR_NONE,ALL_OR_NONE_DO_NOT_REDUCE,DO_NOT_REDUCE} |
| `--stop-link-basis` | string | - | no | Trailing stop reference price (LAST, BID, ASK, MARK) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--stop-link-type` | string | - | no | Trailing stop offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--stop-offset` | float64 | 0 | no | Trailing stop offset amount |
| `--stop-price` | float64 | 0 | no | Stop price |
| `--stop-type` | string | - | no | Trailing stop trigger type (STANDARD, BID, ASK, LAST, MARK) {,ASK,BID,LAST,MARK,STANDARD} |
| `--symbol` | string | - | yes | Equity symbol |
| `--type` | string | - | no | Order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |

**Example:**

```bash
# Buy 10 shares at market price
  schwab-agent order place equity --symbol AAPL --action BUY --quantity 10
  # Buy with a limit price, good till cancel
  schwab-agent order place equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150 --duration GTC
  # Sell with a trailing stop ($2.50 offset)
  schwab-agent order place equity --symbol AAPL --action SELL --quantity 10 --type TRAILING_STOP --stop-offset 2.50
  # Sell with a stop-limit order
  schwab-agent order place equity --symbol AAPL --action SELL --quantity 10 --type STOP_LIMIT --stop-price 145 --price 144
```

#### `schwab-agent order place oco`

Place a one-cancels-other order for an existing position. When one exit fills,
the other is automatically canceled. At least one of --take-profit or --stop-loss
is required. For long positions use SELL; for short positions use BUY. Unlike
bracket orders, OCO has no entry leg.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Exit action (SELL to close long, BUY to close short) {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--stop-loss` | float64 | 0 | no | Stop-loss exit price (stop order) |
| `--symbol` | string | - | yes | Equity symbol |
| `--take-profit` | float64 | 0 | no | Take-profit exit price (limit order) |

**Example:**

```bash
# Set take-profit and stop-loss for a long position
  schwab-agent order place oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140
  # Protect a position with only a stop-loss
  schwab-agent order place oco --symbol AAPL --action SELL --quantity 50 --stop-loss 140
  # Close a short position with exits
  schwab-agent order place oco --symbol TSLA --action BUY --quantity 10 --take-profit 200 --stop-loss 250
```

#### `schwab-agent order place option`

Place a single-leg option order. Requires --underlying, --expiration, --strike,
and exactly one of --call or --put. Use BUY_TO_OPEN/SELL_TO_CLOSE for new
positions and SELL_TO_OPEN/BUY_TO_CLOSE for existing ones. Requires
i-also-like-to-live-dangerously in config for placement.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--call` | bool | false | no | Call option |
| `--destination` | string | - | no | Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO) {,AMEX,AUTO,BATS,BOX,C2,CBOE,ECN_ARCA,INET,ISE,NASDAQ,NYSE,PHLX} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--price` | float64 | 0 | no | Limit price |
| `--price-link-basis` | string | - | no | Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--price-link-type` | string | - | no | Price link offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--put` | bool | false | no | Put option |
| `--quantity` | float64 | 0 | yes | Contract quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--special-instruction` | string | - | no | Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE) {,ALL_OR_NONE,ALL_OR_NONE_DO_NOT_REDUCE,DO_NOT_REDUCE} |
| `--strike` | float64 | 0 | yes | Strike price |
| `--type` | string | - | no | Order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
# Buy a call option to open
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1
  # Sell a put at a limit price
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 190 --put --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 3.50
  # Close an existing call position
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action SELL_TO_CLOSE --quantity 1
```

#### `schwab-agent order preview`

Preview an order from a JSON spec or typed subcommand flags without placing it.
Typed preview subcommands reuse the same local builders as order place, then
return both the built order request and Schwab preview response in one envelope.
This removes the build-then-preview round trip while keeping placement explicit.
Use --save-preview to store the exact reviewed payload locally and return a
previewDigest.digest value for order place --from-preview. Does not require
safety guards since no order is actually placed.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--save-preview` | bool | false | no | Save this preview locally and return a digest for order place --from-preview |
| `--spec` | string | - | yes | Inline JSON, @file, or - for stdin |

**Example:**

```bash
schwab-agent order preview --spec @order.json
  schwab-agent order preview equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --save-preview
  schwab-agent order preview option --underlying AAPL --expiration 2026-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order preview --spec -
```

#### `schwab-agent order preview bracket`

Preview a bracket order without placing it. At least one of --take-profit or
--stop-loss is required. The preview response includes the locally built trigger
order and Schwab's validation, fee, and commission details.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--price` | float64 | 0 | no | Entry price |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--save-preview` | bool | false | no | Save this preview locally and return a digest for order place --from-preview |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--stop-loss` | float64 | 0 | no | Stop-loss exit price |
| `--symbol` | string | - | yes | Equity symbol |
| `--take-profit` | float64 | 0 | no | Take-profit exit price |
| `--type` | string | - | no | Entry order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |

**Example:**

```bash
schwab-agent order preview bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120
	  schwab-agent order preview bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 180 --stop-loss 170
```

#### `schwab-agent order preview buy-with-stop`

Preview a buy-with-stop bracket order through the Schwab API without placing it.
The entry fills first as a TRIGGER parent, then the stop-loss activates automatically.
Optional --take-profit adds a second exit leg, creating an OCO structure where one exit fill cancels the other.
Exit legs are always GOOD_TILL_CANCEL regardless of --duration, which only controls the entry order.
IMPORTANT: Bracket orders cannot be modified via order replace. To adjust, cancel the entire order and re-place with new parameters.
MARKET entry (--type MARKET) skips price validation for stop-loss placement because there is no fixed entry price.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--duration` | string | - | no | Entry duration (exit legs are always GTC) {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--price` | float64 | 0 | no | Entry limit price (required for LIMIT orders, omit for MARKET) |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--save-preview` | bool | false | no | Save this preview locally and return a digest for order place --from-preview |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--stop-loss` | float64 | 0 | yes | Stop trigger price - becomes market sell when hit |
| `--symbol` | string | - | yes | Stock symbol (e.g., AAPL) |
| `--take-profit` | float64 | 0 | no | Optional take-profit limit price |
| `--type` | string | - | no | Entry order type (LIMIT or MARKET, default LIMIT) {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |

**Example:**

```bash
# Buy 10 shares of AAPL at $150 with stop-loss at $140
  schwab-agent order preview buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140

  # Market entry with stop-loss (no price validation)
  schwab-agent order preview buy-with-stop --symbol AAPL --quantity 10 --type MARKET --stop-loss 140

  # With take-profit (creates OCO exit structure)
  schwab-agent order preview buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140 --take-profit 170
```

#### `schwab-agent order preview equity`

Preview an equity (stock) order without placing it. Supports the same flags
as order place equity, but skips the mutable-operation safety gate because no
order is submitted. The response includes the built order request plus Schwab's
preview details so agents can inspect both in one call. Add --save-preview to
return a digest for exact-payload placement.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--activation-price` | float64 | 0 | no | Price that activates the trailing stop |
| `--destination` | string | - | no | Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO) {,AMEX,AUTO,BATS,BOX,C2,CBOE,ECN_ARCA,INET,ISE,NASDAQ,NYSE,PHLX} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--price` | float64 | 0 | no | Limit price |
| `--price-link-basis` | string | - | no | Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--price-link-type` | string | - | no | Price link offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--save-preview` | bool | false | no | Save this preview locally and return a digest for order place --from-preview |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--special-instruction` | string | - | no | Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE) {,ALL_OR_NONE,ALL_OR_NONE_DO_NOT_REDUCE,DO_NOT_REDUCE} |
| `--stop-link-basis` | string | - | no | Trailing stop reference price (LAST, BID, ASK, MARK) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--stop-link-type` | string | - | no | Trailing stop offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--stop-offset` | float64 | 0 | no | Trailing stop offset amount |
| `--stop-price` | float64 | 0 | no | Stop price |
| `--stop-type` | string | - | no | Trailing stop trigger type (STANDARD, BID, ASK, LAST, MARK) {,ASK,BID,LAST,MARK,STANDARD} |
| `--symbol` | string | - | yes | Equity symbol |
| `--type` | string | - | no | Order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |

**Example:**

```bash
schwab-agent order preview equity --symbol AAPL --action BUY --quantity 10
	  schwab-agent order preview equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150 --duration GTC
```

#### `schwab-agent order preview oco`

Preview a one-cancels-other order for an existing position without placing it.
When both exits are present, the built order shows the OCO relationship Schwab
will validate during preview.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Exit action (SELL to close long, BUY to close short) {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--save-preview` | bool | false | no | Save this preview locally and return a digest for order place --from-preview |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--stop-loss` | float64 | 0 | no | Stop-loss exit price (stop order) |
| `--symbol` | string | - | yes | Equity symbol |
| `--take-profit` | float64 | 0 | no | Take-profit exit price (limit order) |

**Example:**

```bash
schwab-agent order preview oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140
	  schwab-agent order preview oco --symbol TSLA --action BUY --quantity 10 --stop-loss 250
```

#### `schwab-agent order preview option`

Preview a single-leg option order without placing it. Requires --underlying,
--expiration, --strike, and exactly one of --call or --put. The response includes
the locally built OCC order request and Schwab's preview response. Add
--save-preview to return a digest for exact-payload placement.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--call` | bool | false | no | Call option |
| `--destination` | string | - | no | Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO) {,AMEX,AUTO,BATS,BOX,C2,CBOE,ECN_ARCA,INET,ISE,NASDAQ,NYSE,PHLX} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--price` | float64 | 0 | no | Limit price |
| `--price-link-basis` | string | - | no | Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--price-link-type` | string | - | no | Price link offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--put` | bool | false | no | Put option |
| `--quantity` | float64 | 0 | yes | Contract quantity |
| `--save-preview` | bool | false | no | Save this preview locally and return a digest for order place --from-preview |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--special-instruction` | string | - | no | Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE) {,ALL_OR_NONE,ALL_OR_NONE_DO_NOT_REDUCE,DO_NOT_REDUCE} |
| `--strike` | float64 | 0 | yes | Strike price |
| `--type` | string | - | no | Order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order preview option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1
	  schwab-agent order preview option --underlying AAPL --expiration 2025-06-20 --strike 190 --put --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 3.50
```

#### `schwab-agent order replace`

Replace an existing order with a new equity order. The original order is
canceled and a new one is created with a new ID. Only equity order flags are
accepted. Requires the "i-also-like-to-live-dangerously" config flag. The
original order status becomes REPLACED after the new order is created.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--activation-price` | float64 | 0 | no | Price that activates the trailing stop |
| `--destination` | string | - | no | Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO) {,AMEX,AUTO,BATS,BOX,C2,CBOE,ECN_ARCA,INET,ISE,NASDAQ,NYSE,PHLX} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--order-id` | string | - | no | Order ID |
| `--price` | float64 | 0 | no | Limit price |
| `--price-link-basis` | string | - | no | Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--price-link-type` | string | - | no | Price link offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--quantity` | float64 | 0 | yes | Share quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--special-instruction` | string | - | no | Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE) {,ALL_OR_NONE,ALL_OR_NONE_DO_NOT_REDUCE,DO_NOT_REDUCE} |
| `--stop-link-basis` | string | - | no | Trailing stop reference price (LAST, BID, ASK, MARK) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--stop-link-type` | string | - | no | Trailing stop offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--stop-offset` | float64 | 0 | no | Trailing stop offset amount |
| `--stop-price` | float64 | 0 | no | Stop price |
| `--stop-type` | string | - | no | Trailing stop trigger type (STANDARD, BID, ASK, LAST, MARK) {,ASK,BID,LAST,MARK,STANDARD} |
| `--symbol` | string | - | yes | Equity symbol |
| `--type` | string | - | no | Order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |

**Example:**

```bash
schwab-agent order replace 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY
   schwab-agent order replace --order-id 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY
```

#### `schwab-agent order replace option`

Replace an existing order with a new single-leg option order. The option
contract is built from --underlying, --expiration, --strike, and exactly one of
--call or --put, then submitted through Schwab's replace endpoint. Requires the
"i-also-like-to-live-dangerously" config flag.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--action` | string | - | yes | Order action {,BUY,BUY_TO_CLOSE,BUY_TO_COVER,BUY_TO_OPEN,EXCHANGE,SELL,SELL_SHORT,SELL_SHORT_EXEMPT,SELL_TO_CLOSE,SELL_TO_OPEN} |
| `--call` | bool | false | no | Call option |
| `--destination` | string | - | no | Order routing destination (INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO) {,AMEX,AUTO,BATS,BOX,C2,CBOE,ECN_ARCA,INET,ISE,NASDAQ,NYSE,PHLX} |
| `--duration` | string | - | no | Order duration {,DAY,END_OF_MONTH,END_OF_WEEK,FILL_OR_KILL,GOOD_TILL_CANCEL,IMMEDIATE_OR_CANCEL,NEXT_END_OF_MONTH} |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--order-id` | string | - | no | Order ID |
| `--price` | float64 | 0 | no | Limit price |
| `--price-link-basis` | string | - | no | Price link reference price (MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE) {,ASK,ASK_BID,AVERAGE,BASE,BID,LAST,MANUAL,MARK,TRIGGER} |
| `--price-link-type` | string | - | no | Price link offset type (VALUE, PERCENT, TICK) {,PERCENT,TICK,VALUE} |
| `--put` | bool | false | no | Put option |
| `--quantity` | float64 | 0 | yes | Contract quantity |
| `--session` | string | - | no | Trading session {,AM,NORMAL,PM,SEAMLESS} |
| `--special-instruction` | string | - | no | Special instruction (ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE) {,ALL_OR_NONE,ALL_OR_NONE_DO_NOT_REDUCE,DO_NOT_REDUCE} |
| `--strike` | float64 | 0 | yes | Strike price |
| `--type` | string | - | no | Order type {,LIMIT,LIMIT_ON_CLOSE,MARKET,MARKET_ON_CLOSE,NET_CREDIT,NET_DEBIT,NET_ZERO,STOP,STOP_LIMIT,TRAILING_STOP,TRAILING_STOP_LIMIT} |
| `--underlying` | string | - | yes | Underlying symbol |

**Example:**

```bash
schwab-agent order replace option 1234567890 --underlying AAPL --expiration 2026-06-19 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00
   schwab-agent order replace option --order-id 1234567890 --underlying AAPL --expiration 2026-06-19 --strike 190 --put --instruction SELL_TO_OPEN --quantity 1 --order-type LIMIT --price 3.50
```

#### `schwab-agent position`

View positions across accounts

#### `schwab-agent position list`

List positions as a flat list with account identifiers and computed cost basis
and P&L fields that Schwab's API does not provide directly. Uses the default
account unless --account or --all-accounts is specified. Computed fields
include totalCostBasis, unrealizedPnL, and unrealizedPnLPct. Apply local symbol,
P&L, and sort filters to shrink output for portfolio triage workflows.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--all-accounts` | bool | false | no | Show positions across all linked accounts |
| `--losers-only` | bool | false | no | Show only positions with negative unrealized P&L |
| `--max-pnl` | float64 | 0 | no | Maximum unrealized P&L to include when set |
| `--min-pnl` | float64 | 0 | no | Minimum unrealized P&L to include when set |
| `--sort` | string | - | no | Sort positions by pnl-desc, pnl-asc, or value-desc {,pnl-asc,pnl-desc,value-desc} |
| `--symbol` | stringSlice | [] | no | Filter by symbol (repeatable, comma-separated values allowed) |

**Example:**

```bash
schwab-agent position list
	  schwab-agent position list --account ABCDEF1234567890
	  schwab-agent position list --all-accounts
	  schwab-agent position list --symbol AAPL --symbol MSFT
	  schwab-agent position list --losers-only --sort pnl-asc
	  schwab-agent position list --min-pnl -100 --max-pnl 500 --sort value-desc
```

#### `schwab-agent quote`

Get quotes for one or more symbols

#### `schwab-agent quote get`

Get quotes for one or more symbols. Multiple symbols return a partial response
when some are missing. Use --fields to select specific data groups (quote,
fundamental, extended, reference, regular). Key output fields include last
price, bid/ask, volume, 52-week high/low, PE ratio, dividend yield, and
market cap.

Use --underlying, --expiration, --strike, and --call/--put to quote an option
by its components instead of providing a raw OCC symbol. These flags are
mutually exclusive with positional symbol arguments.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call` | bool | false | no | Call option |
| `--expiration` | string | - | no | Expiration date (YYYY-MM-DD) for option quote |
| `--fields` | stringSlice | [] | no | Quote fields to return (repeatable): quote, fundamental, extended, reference, regular |
| `--indicative` | bool | false | no | Request indicative (non-tradeable) quotes |
| `--put` | bool | false | no | Put option |
| `--strike` | float64 | 0 | no | Strike price for option quote |
| `--underlying` | string | - | no | Underlying symbol for option quote |

**Example:**

```bash
schwab-agent quote get AAPL
  schwab-agent quote get AAPL NVDA TSLA
  schwab-agent quote get AAPL --fields quote,fundamental
  schwab-agent quote get AAPL --fields fundamental --indicative
  schwab-agent quote get --underlying AMZN --expiration 2026-05-15 --strike 270 --call
```

#### `schwab-agent symbol`

Build and parse OCC-format option symbols locally. These are pure computation
commands that make no API calls and require no authentication.

#### `schwab-agent symbol build`

Build an OCC option symbol from underlying, expiration date, strike price, and
contract type. No API call or authentication required. Output is JSON with the
constructed symbol and its components.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--call` | bool | false | no | Call option |
| `--expiration` | string | - | yes | Expiration date (YYYY-MM-DD) |
| `--put` | bool | false | no | Put option |
| `--strike` | float64 | 0 | yes | Strike price (e.g. 200, 450.50) |
| `--underlying` | string | - | yes | Underlying symbol (e.g. AAPL) |

**Example:**

```bash
schwab-agent symbol build --underlying AAPL --expiration 2025-06-20 --strike 200 --call
  schwab-agent symbol build --underlying TSLA --expiration 2025-12-19 --strike 350.50 --put
```

#### `schwab-agent symbol parse`

Parse an OCC option symbol string into its underlying, expiration date, strike
price, and contract type components. No API call or authentication required.

**Example:**

```bash
schwab-agent symbol parse "AAPL  250620C00200000"
```

#### `schwab-agent ta`

Compute technical analysis indicators from Schwab price history. All indicators
are calculated locally after fetching candles. Results are always keyed by
symbol under data.<SYMBOL>, even when one symbol is requested, so agents can use
the same parser for single-symbol and batch analysis. Use --interval to select
data frequency (daily, weekly, 1min, 5min, 15min, 30min). Time-series commands
default --points to 1 for token-efficient latest-value output; use --points 0
when you need the full computed series.

#### `schwab-agent ta adx`

Compute Average Directional Index for a symbol. ADX above 25 indicates a
trending market, below 20 indicates a ranging market. Output includes adx,
plus_di, and minus_di for directional bias. Default period is 14.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--period` | int | 14 | no | Indicator period |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |

**Example:**

```bash
schwab-agent ta adx AAPL
  schwab-agent ta adx AAPL --period 14 --interval daily --points 10
```

#### `schwab-agent ta atr`

Compute Average True Range for a symbol. ATR measures volatility in price
units: higher values indicate wider price swings. Uses high, low, and close
data with Wilder smoothing. Default period is 14.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--period` | int | 14 | no | Indicator period |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |

**Example:**

```bash
schwab-agent ta atr AAPL
  schwab-agent ta atr AAPL --period 14 --interval daily --points 10
```

#### `schwab-agent ta bbands`

Compute Bollinger Bands for a symbol. Output keys are upper, middle, and lower
bands. Price near the upper band is relatively high, near the lower band is
relatively low. Use --std-dev to widen or narrow the bands (default 2.0).

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--period` | int | 20 | no | BBands period |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |
| `--std-dev` | float64 | 2.0 | no | Standard deviations |

**Example:**

```bash
schwab-agent ta bbands AAPL
  schwab-agent ta bbands AAPL --period 20 --std-dev 2.0 --points 10
  schwab-agent ta bbands AAPL --std-dev 1.5 --points 10
```

#### `schwab-agent ta dashboard`

Compute an opinionated technical-analysis dashboard for a symbol with one
price-history fetch. The dashboard includes SMA 21/50/200 trend context, RSI 14,
MACD 12/26/9, ATR 14, Bollinger Bands 20/2, volume context, and 20/252-candle
high-low ranges. Signals are neutral factual labels for agents, not trading advice.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |

**Example:**

```bash
schwab-agent ta dashboard AAPL
  schwab-agent ta dashboard AAPL --points 5
  schwab-agent ta dashboard NVDA --interval weekly --points 10
```

#### `schwab-agent ta ema`

Compute Exponential Moving Average for a symbol. EMA gives more weight to
recent prices than SMA. The --period flag can be repeated or comma-separated
for multiple lookbacks in one command (e.g. --period 12,26 for crossover
analysis).

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--period` | intSlice | 20 | no | Indicator period (repeatable or comma-separated) |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |

**Example:**

```bash
schwab-agent ta ema AAPL
  schwab-agent ta ema AAPL --period 50 --interval daily --points 10
  schwab-agent ta ema AAPL --period 12,26 --points 5
```

#### `schwab-agent ta expected-move`

Compute expected price move from ATM straddle pricing. Fetches the option
chain, finds the nearest expiration to --dte (default 30), and prices the
ATM straddle. Output includes 1x and 2x standard deviation ranges (upper
and lower bounds).

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--dte` | int | 30 | no | Target days to expiration |

**Example:**

```bash
schwab-agent ta expected-move AAPL
  schwab-agent ta expected-move AAPL --dte 45
```

#### `schwab-agent ta hv`

Compute Historical Volatility for a symbol. Returns a scalar summary (not
time series) with annualized volatility, percentile rank, and regime
classification (low, normal, high, extreme). Includes daily, weekly, and
monthly volatility breakdowns.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--period` | int | 20 | no | Indicator period |

**Example:**

```bash
schwab-agent ta hv AAPL
  schwab-agent ta hv AAPL --period 20 --interval daily
```

#### `schwab-agent ta macd`

Compute MACD (Moving Average Convergence/Divergence) for a symbol. Uses
--fast, --slow, and --signal periods instead of --period. Output keys include
macd, signal, and histogram (histogram = macd - signal). Defaults: fast=12,
slow=26, signal=9.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--fast` | int | 12 | no | Fast EMA period |
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |
| `--signal` | int | 9 | no | Signal EMA period |
| `--slow` | int | 26 | no | Slow EMA period |

**Example:**

```bash
schwab-agent ta macd AAPL
  schwab-agent ta macd AAPL --fast 12 --slow 26 --signal 9 --points 10
  schwab-agent ta macd NVDA --interval weekly --points 10
```

#### `schwab-agent ta rsi`

Compute Relative Strength Index for a symbol. RSI ranges 0-100: values above
70 suggest overbought conditions, below 30 suggest oversold. The --period flag
can be repeated or comma-separated for multiple lookbacks. Default period is 14.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--period` | intSlice | 14 | no | Indicator period (repeatable or comma-separated) |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |

**Example:**

```bash
schwab-agent ta rsi AAPL
  schwab-agent ta rsi AAPL --period 14 --interval daily --points 10
  schwab-agent ta rsi AAPL --period 9 --interval 5min --points 20
```

#### `schwab-agent ta sma`

Compute Simple Moving Average for a symbol. The --period flag sets the lookback
window (default 20). Multiple periods can be requested in one command using
comma-separated values or repeated flags, producing crossover-ready output
with keys like sma_21, sma_50, sma_200.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--period` | intSlice | 20 | no | Indicator period (repeatable or comma-separated) |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |

**Example:**

```bash
schwab-agent ta sma AAPL
  schwab-agent ta sma AAPL --period 50 --interval daily --points 10
  schwab-agent ta sma AAPL --period 21,50,200 --points 1
```

#### `schwab-agent ta stoch`

Compute Stochastic Oscillator for a symbol. Output keys are slowk and slowd,
both ranging 0-100. Above 80 and below 20 are common signal thresholds.
Configurable via --k-period, --smooth-k, and --d-period.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--d-period` | int | 3 | no | Slow %D period |
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--k-period` | int | 14 | no | Fast %K lookback period |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |
| `--smooth-k` | int | 3 | no | Slow %K smoothing period |

**Example:**

```bash
schwab-agent ta stoch AAPL
  schwab-agent ta stoch AAPL --k-period 14 --smooth-k 3 --d-period 3 --points 10
```

#### `schwab-agent ta vwap`

Compute Volume Weighted Average Price for a symbol. VWAP is a cumulative
indicator with no --period flag. Primarily used for intraday analysis with
minute-level intervals. Price above VWAP suggests bullish bias, below
suggests bearish.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--interval` | string | daily | no | Data interval (daily, weekly, 1min, 5min, 15min, 30min) |
| `--points` | int | 1 | no | Number of output points (default 1; 0 = all) |

**Example:**

```bash
schwab-agent ta vwap AAPL
  schwab-agent ta vwap AAPL --interval 5min --points 20
```

### Environment Variable Prefix

All environment variables use the `SCHWAB_AGENT_` prefix.

### Examples

#### schwab-agent account get

```bash
schwab-agent account get
  schwab-agent account get ABCDEF1234567890
  schwab-agent account get --positions
```

#### schwab-agent account list

```bash
schwab-agent account list
  schwab-agent account list --positions
```

#### schwab-agent account numbers

```bash
schwab-agent account numbers
```

#### schwab-agent account resolve

```bash
# Resolve by nickname
  schwab-agent account resolve "My IRA"

  # Resolve by account number
  schwab-agent account resolve 12345678

  # Resolve using the --account flag
  schwab-agent account resolve --account "My IRA"

  # Resolve using the default account from config
  schwab-agent account resolve
```

#### schwab-agent account set-default

```bash
schwab-agent account set-default ABCDEF1234567890
```

#### schwab-agent account summary

```bash
schwab-agent account summary
  schwab-agent account summary --positions
```

#### schwab-agent account transaction get

```bash
schwab-agent account transaction get 98765432101
```

#### schwab-agent account transaction list

```bash
schwab-agent account transaction list
  schwab-agent account transaction list --types TRADE --symbol AAPL
  schwab-agent account transaction list --from 2025-01-01T00:00:00Z --to 2025-01-31T23:59:59Z
```

#### schwab-agent analyze

```bash
schwab-agent analyze AAPL
  schwab-agent analyze AAPL NVDA
  schwab-agent analyze AAPL --interval weekly --points 5
```

#### schwab-agent auth login

```bash
schwab-agent auth login
  schwab-agent auth login --no-browser
```

#### schwab-agent auth refresh

```bash
schwab-agent auth refresh
```

#### schwab-agent auth status

```bash
schwab-agent auth status
```

#### schwab-agent chain expiration

```bash
schwab-agent chain expiration AAPL
```

#### schwab-agent chain get

```bash
schwab-agent chain get AAPL
  schwab-agent chain get AAPL --type CALL --strike-count 5
  schwab-agent chain get AAPL --from-date 2025-06-01 --to-date 2025-07-31 --type PUT
  schwab-agent chain get AAPL --strategy ANALYTICAL --volatility 30.5 --days-to-expiration 45
  schwab-agent chain get AAPL --strike-range NTM --include-underlying-quote
```

#### schwab-agent history get

```bash
schwab-agent history get AAPL
  schwab-agent history get AAPL --period-type month --period 3 --frequency-type daily --frequency 1
  schwab-agent history get AAPL --period-type day --period 5 --frequency-type minute --frequency 15
  schwab-agent history get AAPL --from 1735689600000 --to 1743379200000
```

#### schwab-agent indicators

```bash
schwab-agent indicators AAPL
  schwab-agent indicators AAPL --points 5
  schwab-agent indicators NVDA --interval weekly --points 10
  schwab-agent indicators AAPL MSFT NVDA
```

#### schwab-agent instrument get

```bash
schwab-agent instrument get 037833100
```

#### schwab-agent instrument search

```bash
schwab-agent instrument search AAPL
  schwab-agent instrument search "Apple" --projection desc-search
  schwab-agent instrument search AAPL --projection fundamental
```

#### schwab-agent market hours

```bash
schwab-agent market hours
  schwab-agent market hours equity
  schwab-agent market hours equity option
```

#### schwab-agent market movers

```bash
schwab-agent market movers '$SPX' --sort PERCENT_CHANGE_UP
  schwab-agent market movers '$DJI' --sort VOLUME --frequency 5
  schwab-agent market movers EQUITY_ALL --sort PERCENT_CHANGE_UP
```

#### schwab-agent option ticket get

```bash
schwab-agent option ticket get AAPL --expiration 2026-01-16 --strike 200 --call
  schwab-agent option ticket get TSLA --expiration 2026-03-20 --strike 180 --put
```

#### schwab-agent order build back-ratio

```bash
schwab-agent order build back-ratio --underlying F --expiration 2026-06-18 --short-strike 12 --long-strike 14 --call --open --quantity 1 --long-ratio 2 --debit --price 0.20
```

#### schwab-agent order build bracket

```bash
schwab-agent order build bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120
  schwab-agent order build bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 180 --stop-loss 170
```

#### schwab-agent order build butterfly

```bash
schwab-agent order build butterfly --underlying F --expiration 2026-06-18 --lower-strike 10 --middle-strike 12 --upper-strike 14 --call --buy --open --quantity 1 --price 0.50
```

#### schwab-agent order build buy-with-stop

```bash
# Buy 10 shares of AAPL at $150 with stop-loss at $140
  schwab-agent order build buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140

  # Market entry with stop-loss (no price validation)
  schwab-agent order build buy-with-stop --symbol AAPL --quantity 10 --type MARKET --stop-loss 140

  # With take-profit (creates OCO exit structure)
  schwab-agent order build buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140 --take-profit 170
```

#### schwab-agent order build calendar

```bash
schwab-agent order build calendar --underlying F --near-expiration 2026-05-16 --far-expiration 2026-07-17 --strike 12 --call --open --quantity 1 --price 0.50
```

#### schwab-agent order build collar

```bash
schwab-agent order build collar --underlying F --put-strike 10 --call-strike 14 --expiration 2026-06-18 --quantity 1 --open --price 12.00
```

#### schwab-agent order build condor

```bash
schwab-agent order build condor --underlying F --expiration 2026-06-18 --lower-strike 10 --lower-middle-strike 12 --upper-middle-strike 14 --upper-strike 16 --call --buy --open --quantity 1 --price 0.75
```

#### schwab-agent order build covered-call

```bash
schwab-agent order build covered-call --underlying F --expiration 2026-06-18 --strike 14 --quantity 1 --price 12.00
```

#### schwab-agent order build diagonal

```bash
schwab-agent order build diagonal --underlying F --near-strike 12 --far-strike 14 --near-expiration 2026-05-16 --far-expiration 2026-07-17 --call --open --quantity 1 --price 0.50
```

#### schwab-agent order build double-diagonal

```bash
schwab-agent order build double-diagonal --underlying F --near-expiration 2026-06-18 --far-expiration 2026-07-17 --put-far-strike 9 --put-near-strike 10 --call-near-strike 14 --call-far-strike 15 --open --quantity 1 --price 0.80
```

#### schwab-agent order build equity

```bash
schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --duration DAY
  schwab-agent order build equity --symbol AAPL --action SELL --quantity 10 --type STOP --stop-price 145
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order place --spec -
```

#### schwab-agent order build fts

```bash
schwab-agent order build fts --primary @entry.json --secondary @exit.json
  schwab-agent order build fts --primary '{"orderType":"LIMIT",...}' --secondary '{"orderType":"STOP",...}'
```

#### schwab-agent order build iron-condor

```bash
schwab-agent order build iron-condor --underlying F --expiration 2026-06-18 --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 --open --quantity 1 --price 0.50
```

#### schwab-agent order build oco

```bash
schwab-agent order build oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140
  schwab-agent order build oco --symbol TSLA --action BUY --quantity 10 --stop-loss 250
```

#### schwab-agent order build option

```bash
schwab-agent order build option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00
  schwab-agent order build option --underlying AAPL --expiration 2025-06-20 --strike 190 --put --action SELL_TO_OPEN --quantity 1
```

#### schwab-agent order build straddle

```bash
schwab-agent order build straddle --underlying F --expiration 2026-06-18 --strike 12 --buy --open --quantity 1 --price 1.50
  schwab-agent order build straddle --underlying F --expiration 2026-06-18 --strike 12 --sell --open --quantity 1 --price 1.50
```

#### schwab-agent order build strangle

```bash
schwab-agent order build strangle --underlying F --expiration 2026-06-18 --call-strike 14 --put-strike 10 --buy --open --quantity 1 --price 0.50
  schwab-agent order build strangle --underlying F --expiration 2026-06-18 --call-strike 14 --put-strike 10 --sell --open --quantity 1 --price 0.50
```

#### schwab-agent order build vertical

```bash
# Bull call spread
  schwab-agent order build vertical --underlying F --expiration 2026-06-18 --long-strike 12 --short-strike 14 --call --open --quantity 1 --price 0.50
  # Bear put spread
  schwab-agent order build vertical --underlying F --expiration 2026-06-18 --long-strike 14 --short-strike 12 --put --open --quantity 1 --price 0.50
```

#### schwab-agent order build vertical-roll

```bash
schwab-agent order build vertical-roll --underlying F --close-expiration 2026-06-18 --open-expiration 2026-07-17 --close-long-strike 12 --close-short-strike 14 --open-long-strike 13 --open-short-strike 15 --call --debit --quantity 1 --price 0.25
```

#### schwab-agent order cancel

```bash
schwab-agent order cancel 1234567890
   schwab-agent order cancel --order-id 1234567890
```

#### schwab-agent order get

```bash
schwab-agent order get 1234567890
  schwab-agent order get --order-id 1234567890
```

#### schwab-agent order list

```bash
schwab-agent order list
  schwab-agent order list --recent
  schwab-agent order list --status all
  schwab-agent order list --status WORKING --status PENDING_ACTIVATION
  schwab-agent order list --status WORKING,FILLED,EXPIRED
  schwab-agent order list --from 2025-01-01 --to 2025-01-31
```

#### schwab-agent order place

```bash
# Place from a JSON file
  schwab-agent order place --spec @order.json
  # Place the exact payload saved by a previous preview
  schwab-agent order preview equity --account abc123 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --save-preview
  schwab-agent order place --from-preview <digest>
  # Place from stdin (piped from order build)
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order place --spec -
  # Place from inline JSON
  schwab-agent order place --spec '{"orderType":"LIMIT",...}'
```

#### schwab-agent order place bracket

```bash
# Buy with both take-profit and stop-loss
  schwab-agent order place bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120
  # Buy with only a stop-loss safety net
  schwab-agent order place bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 180 --stop-loss 170
  # Buy with only a take-profit target
  schwab-agent order place bracket --symbol TSLA --action BUY --quantity 5 --type MARKET --take-profit 300
```

#### schwab-agent order place buy-with-stop

```bash
# Buy 10 shares of AAPL at $150 with stop-loss at $140
  schwab-agent order place buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140

  # Market entry with stop-loss (no price validation)
  schwab-agent order place buy-with-stop --symbol AAPL --quantity 10 --type MARKET --stop-loss 140

  # With take-profit (creates OCO exit structure)
  schwab-agent order place buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140 --take-profit 170
```

#### schwab-agent order place equity

```bash
# Buy 10 shares at market price
  schwab-agent order place equity --symbol AAPL --action BUY --quantity 10
  # Buy with a limit price, good till cancel
  schwab-agent order place equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150 --duration GTC
  # Sell with a trailing stop ($2.50 offset)
  schwab-agent order place equity --symbol AAPL --action SELL --quantity 10 --type TRAILING_STOP --stop-offset 2.50
  # Sell with a stop-limit order
  schwab-agent order place equity --symbol AAPL --action SELL --quantity 10 --type STOP_LIMIT --stop-price 145 --price 144
```

#### schwab-agent order place oco

```bash
# Set take-profit and stop-loss for a long position
  schwab-agent order place oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140
  # Protect a position with only a stop-loss
  schwab-agent order place oco --symbol AAPL --action SELL --quantity 50 --stop-loss 140
  # Close a short position with exits
  schwab-agent order place oco --symbol TSLA --action BUY --quantity 10 --take-profit 200 --stop-loss 250
```

#### schwab-agent order place option

```bash
# Buy a call option to open
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1
  # Sell a put at a limit price
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 190 --put --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 3.50
  # Close an existing call position
  schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action SELL_TO_CLOSE --quantity 1
```

#### schwab-agent order preview

```bash
schwab-agent order preview --spec @order.json
  schwab-agent order preview equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --save-preview
  schwab-agent order preview option --underlying AAPL --expiration 2026-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00
  schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order preview --spec -
```

#### schwab-agent order preview bracket

```bash
schwab-agent order preview bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120
	  schwab-agent order preview bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 180 --stop-loss 170
```

#### schwab-agent order preview buy-with-stop

```bash
# Buy 10 shares of AAPL at $150 with stop-loss at $140
  schwab-agent order preview buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140

  # Market entry with stop-loss (no price validation)
  schwab-agent order preview buy-with-stop --symbol AAPL --quantity 10 --type MARKET --stop-loss 140

  # With take-profit (creates OCO exit structure)
  schwab-agent order preview buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140 --take-profit 170
```

#### schwab-agent order preview equity

```bash
schwab-agent order preview equity --symbol AAPL --action BUY --quantity 10
	  schwab-agent order preview equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 150 --duration GTC
```

#### schwab-agent order preview oco

```bash
schwab-agent order preview oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140
	  schwab-agent order preview oco --symbol TSLA --action BUY --quantity 10 --stop-loss 250
```

#### schwab-agent order preview option

```bash
schwab-agent order preview option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1
	  schwab-agent order preview option --underlying AAPL --expiration 2025-06-20 --strike 190 --put --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 3.50
```

#### schwab-agent order replace

```bash
schwab-agent order replace 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY
   schwab-agent order replace --order-id 1234567890 --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 155.00 --duration DAY
```

#### schwab-agent order replace option

```bash
schwab-agent order replace option 1234567890 --underlying AAPL --expiration 2026-06-19 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00
   schwab-agent order replace option --order-id 1234567890 --underlying AAPL --expiration 2026-06-19 --strike 190 --put --instruction SELL_TO_OPEN --quantity 1 --order-type LIMIT --price 3.50
```

#### schwab-agent position list

```bash
schwab-agent position list
	  schwab-agent position list --account ABCDEF1234567890
	  schwab-agent position list --all-accounts
	  schwab-agent position list --symbol AAPL --symbol MSFT
	  schwab-agent position list --losers-only --sort pnl-asc
	  schwab-agent position list --min-pnl -100 --max-pnl 500 --sort value-desc
```

#### schwab-agent quote get

```bash
schwab-agent quote get AAPL
  schwab-agent quote get AAPL NVDA TSLA
  schwab-agent quote get AAPL --fields quote,fundamental
  schwab-agent quote get AAPL --fields fundamental --indicative
  schwab-agent quote get --underlying AMZN --expiration 2026-05-15 --strike 270 --call
```

#### schwab-agent symbol build

```bash
schwab-agent symbol build --underlying AAPL --expiration 2025-06-20 --strike 200 --call
  schwab-agent symbol build --underlying TSLA --expiration 2025-12-19 --strike 350.50 --put
```

#### schwab-agent symbol parse

```bash
schwab-agent symbol parse "AAPL  250620C00200000"
```

#### schwab-agent ta adx

```bash
schwab-agent ta adx AAPL
  schwab-agent ta adx AAPL --period 14 --interval daily --points 10
```

#### schwab-agent ta atr

```bash
schwab-agent ta atr AAPL
  schwab-agent ta atr AAPL --period 14 --interval daily --points 10
```

#### schwab-agent ta bbands

```bash
schwab-agent ta bbands AAPL
  schwab-agent ta bbands AAPL --period 20 --std-dev 2.0 --points 10
  schwab-agent ta bbands AAPL --std-dev 1.5 --points 10
```

#### schwab-agent ta dashboard

```bash
schwab-agent ta dashboard AAPL
  schwab-agent ta dashboard AAPL --points 5
  schwab-agent ta dashboard NVDA --interval weekly --points 10
```

#### schwab-agent ta ema

```bash
schwab-agent ta ema AAPL
  schwab-agent ta ema AAPL --period 50 --interval daily --points 10
  schwab-agent ta ema AAPL --period 12,26 --points 5
```

#### schwab-agent ta expected-move

```bash
schwab-agent ta expected-move AAPL
  schwab-agent ta expected-move AAPL --dte 45
```

#### schwab-agent ta hv

```bash
schwab-agent ta hv AAPL
  schwab-agent ta hv AAPL --period 20 --interval daily
```

#### schwab-agent ta macd

```bash
schwab-agent ta macd AAPL
  schwab-agent ta macd AAPL --fast 12 --slow 26 --signal 9 --points 10
  schwab-agent ta macd NVDA --interval weekly --points 10
```

#### schwab-agent ta rsi

```bash
schwab-agent ta rsi AAPL
  schwab-agent ta rsi AAPL --period 14 --interval daily --points 10
  schwab-agent ta rsi AAPL --period 9 --interval 5min --points 20
```

#### schwab-agent ta sma

```bash
schwab-agent ta sma AAPL
  schwab-agent ta sma AAPL --period 50 --interval daily --points 10
  schwab-agent ta sma AAPL --period 21,50,200 --points 1
```

#### schwab-agent ta stoch

```bash
schwab-agent ta stoch AAPL
  schwab-agent ta stoch AAPL --k-period 14 --smooth-k 3 --d-period 3 --points 10
```

#### schwab-agent ta vwap

```bash
schwab-agent ta vwap AAPL
  schwab-agent ta vwap AAPL --interval 5min --points 20
```

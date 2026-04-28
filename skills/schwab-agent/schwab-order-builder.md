# Order Construction

`order build` outputs raw JSON (not envelope-wrapped) and makes no API call. Pipe output to `order place --spec - --confirm` or `order preview --spec -`.

## Equity Orders

```bash
schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --duration DAY
```

## Option Orders

```bash
schwab-agent order build option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5.00
```

## Bracket Orders

Entry + automatic exit orders. At least one of `--take-profit` or `--stop-loss` required. Exit instructions auto-inverted (BUY entry becomes SELL exit). Cancel cascades to children. Replace not supported for brackets.

```bash
schwab-agent order build bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120
```

## OCO Orders (One-Cancels-Other)

For **existing positions** only (no entry leg). When one exit fills, the other is canceled. At least one of `--take-profit` or `--stop-loss` required.

```bash
schwab-agent order build oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140
```

| | Bracket | OCO |
|---|---|---|
| Entry leg | Yes (buys/sells first) | No (you already own the position) |
| Use case | New position + automatic exits | Protect existing position |
| Structure | TRIGGER -> OCO/SINGLE | OCO (two SINGLE children) or standalone SINGLE |

For shorts, action is BUY and price relationships invert (take-profit below, stop-loss above current price).

## Multi-Leg Option Strategies

All output raw JSON. Place via: `schwab-agent order build <type> ... | schwab-agent order place --spec - --confirm`

### Covered Call

**Selling a call against shares you ALREADY OWN**: use regular option command (`order place option --action SELL_TO_OPEN --call ...`).

**Buying shares AND selling a call in one atomic order** (new position):

```bash
schwab-agent order build covered-call --underlying F --expiration 2026-06-18 --strike 14 --quantity 1 --price 12.00
```

`--quantity 1` = 100 shares + 1 call contract. `--price` = net debit per share after call premium. Always NET_DEBIT, BUY shares + SELL_TO_OPEN call.

### Vertical Spread

```bash
schwab-agent order build vertical --underlying F --expiration 2026-06-18 --long-strike 12 --short-strike 14 --call --open --quantity 1 --price 0.50
```

`--long-strike` = strike bought, `--short-strike` = strike sold. NET_DEBIT or NET_CREDIT auto-determined from strike relationship.

### Iron Condor

Strikes must be ordered: put-long < put-short < call-short < call-long. Opening = NET_CREDIT.

```bash
schwab-agent order build iron-condor --underlying F --expiration 2026-06-18 --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 --open --quantity 1 --price 0.50
```

### Straddle

Same strike, one call + one put.

```bash
schwab-agent order build straddle --underlying F --expiration 2026-06-18 --strike 12 --buy --open --quantity 1 --price 1.50
```

### Strangle

Different strikes, one call + one put.

```bash
schwab-agent order build strangle --underlying F --expiration 2026-06-18 --call-strike 14 --put-strike 10 --buy --open --quantity 1 --price 0.50
```

### Calendar Spread

Same strike, different expirations. Sells near-term, buys far-term.

```bash
schwab-agent order build calendar --underlying F --near-expiration 2026-05-16 --far-expiration 2026-07-17 --strike 12 --call --open --quantity 1 --price 0.50
```

### Diagonal Spread

Different strikes AND different expirations.

```bash
schwab-agent order build diagonal --underlying F --near-strike 12 --far-strike 14 --near-expiration 2026-05-16 --far-expiration 2026-07-17 --call --open --quantity 1 --price 0.50
```

### Collar

Long stock + long put + short call. Limits downside, caps upside.

```bash
schwab-agent order build collar --underlying F --put-strike 10 --call-strike 14 --expiration 2026-06-18 --quantity 1 --open --price 12.00
```

`--quantity 1` = 100 shares + 1 put + 1 call. `--price` = net debit per share.

### First-Triggers-Second (FTS)

Combine two order specs: primary order triggers secondary on fill.

```bash
schwab-agent order build fts --primary @entry.json --secondary @exit.json
schwab-agent order build fts --primary '{"orderType":"LIMIT",...}' --secondary '{"orderType":"STOP",...}'
```

Both `--primary` and `--secondary` required. Accept inline JSON, `@file`, or `-` for stdin.

## Flags Reference

### Core Flags

| Flag | Values | Used with |
|------|--------|-----------|
| `--symbol` | Stock ticker | equity, bracket, oco |
| `--action` | BUY, SELL, BUY_TO_OPEN, BUY_TO_CLOSE, SELL_TO_OPEN, SELL_TO_CLOSE | all |
| `--quantity` | Number of shares/contracts | all |
| `--type` | MARKET, LIMIT, STOP, STOP_LIMIT, TRAILING_STOP | equity, bracket |
| `--price` | Limit price or net debit/credit | LIMIT, STOP_LIMIT, multi-leg |
| `--stop-price` | Stop trigger price | STOP, STOP_LIMIT |
| `--duration` | DAY, GOOD_TILL_CANCEL (or GTC), FILL_OR_KILL (or FOK), IMMEDIATE_OR_CANCEL (or IOC), END_OF_WEEK, END_OF_MONTH, NEXT_END_OF_MONTH | all |
| `--session` | NORMAL, AM, PM, SEAMLESS | all |

### Exit Flags

| Flag | Description | Used with |
|------|-------------|-----------|
| `--take-profit` | Take-profit price (at least one exit required) | bracket, oco |
| `--stop-loss` | Stop-loss price (at least one exit required) | bracket, oco |

### Option Flags

| Flag | Description | Used with |
|------|-------------|-----------|
| `--underlying` | Underlying symbol | option, all multi-leg |
| `--expiration` | Expiration date YYYY-MM-DD | option, vertical, iron-condor, straddle, strangle, covered-call, collar |
| `--strike` | Strike price | option, straddle, covered-call, calendar |
| `--call` / `--put` | Contract type (mutually exclusive) | option, vertical, calendar, diagonal |
| `--buy` / `--sell` | Direction | straddle, strangle |
| `--open` / `--close` | Opening or closing position | vertical, iron-condor, straddle, strangle, calendar, diagonal, collar |
| `--long-strike` / `--short-strike` | Strikes bought/sold | vertical |
| `--call-strike` / `--put-strike` | Per-leg strikes | strangle, iron-condor, collar |
| `--near-strike` / `--far-strike` | Near/far leg strikes | diagonal |
| `--near-expiration` / `--far-expiration` | Near/far leg expirations | calendar, diagonal |
| `--put-long-strike` / `--put-short-strike` | Put wing strikes | iron-condor |
| `--call-short-strike` / `--call-long-strike` | Call wing strikes | iron-condor |

### Trailing Stop Flags

| Flag | Values | Default |
|------|--------|---------|
| `--stop-offset` | Trailing stop offset amount | - |
| `--stop-link-basis` | LAST, BID, ASK, MARK | LAST |
| `--stop-link-type` | VALUE, PERCENT, TICK | VALUE |
| `--stop-type` | STANDARD, BID, ASK, LAST, MARK | STANDARD |
| `--activation-price` | Price that activates the trailing stop | - |

### Advanced Flags

| Flag | Values | Used with |
|------|--------|-----------|
| `--special-instruction` | ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE | equity, option |
| `--destination` | INET, ECN_ARCA, CBOE, AMEX, PHLX, ISE, BOX, NYSE, NASDAQ, BATS, C2, AUTO | equity, option |
| `--price-link-basis` | MANUAL, BASE, TRIGGER, LAST, BID, ASK, ASK_BID, MARK, AVERAGE | equity, option |
| `--price-link-type` | VALUE, PERCENT, TICK | equity, option |

# Order Construction

Build order JSON payloads offline without making API calls. Use these builders to inspect payloads before placing, or pipe output to `order place --spec -` or `order preview --spec -`.

## Building Orders Offline

`order build` outputs raw JSON (not envelope-wrapped) and makes no API call. Useful for inspecting the payload before placing.

```bash
schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --duration DAY
```

Sub-types: equity, option, bracket, oco, vertical, iron-condor, straddle, strangle, covered-call.

## Bracket Orders

Bracket orders combine an entry with one or two automatic exit orders.

### Entry with take-profit + stop-loss

```bash
schwab-agent order place bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120 --confirm
```

### Entry with stop-loss only (protective stop)

Buy a stock and automatically place a stop-loss that activates when the entry fills. No need to wait for fill and manually add a stop.

```bash
schwab-agent order place bracket --symbol NVDA --action BUY --quantity 10 --type LIMIT --price 130 --stop-loss 120 --confirm
```

### Entry with take-profit only

```bash
schwab-agent order place bracket --symbol NVDA --action BUY --quantity 10 --type LIMIT --price 130 --take-profit 150 --confirm
```

### Bracket Internals

Bracket orders use Schwab's TRIGGER order strategy. The structure depends on how many exit conditions are provided:

- **Both exits**: TRIGGER (entry) → OCO → 2 SINGLE (take-profit LIMIT + stop-loss STOP)
- **One exit**: TRIGGER (entry) → SINGLE (stop-loss STOP or take-profit LIMIT)

At least one of `--take-profit` or `--stop-loss` is required. The exit order instruction is automatically inverted (BUY entry → SELL exit, SELL entry → BUY exit).

Canceling a bracket order cascades to all child orders.

Replace is not supported for bracket orders. Cancel and re-place instead.

## OCO Orders (One-Cancels-Other)

OCO orders are for **existing positions**. Unlike bracket orders (which include an entry leg), OCO places exit orders against shares or options you already hold. When one exit fills, the other is automatically canceled.

### Full OCO (take-profit + stop-loss)

```bash
schwab-agent order place oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140 --confirm
```

### Stop-loss only

```bash
schwab-agent order place oco --symbol AAPL --action SELL --quantity 50 --stop-loss 140 --confirm
```

### Take-profit only

```bash
schwab-agent order place oco --symbol AAPL --action SELL --quantity 50 --take-profit 160 --confirm
```

### Closing a short position

For shorts, the action is BUY and price relationships are inverted (take-profit below current price, stop-loss above):

```bash
schwab-agent order place oco --symbol TSLA --action BUY --quantity 25 --take-profit 100 --stop-loss 200 --confirm
```

### OCO vs Bracket

| | Bracket | OCO |
|---|---|---|
| Entry leg | Yes (buys/sells first) | No (you already own the position) |
| Use case | New position + automatic exits | Protect existing position |
| Structure | TRIGGER -> OCO/SINGLE | OCO (two SINGLE children) or standalone SINGLE |

### OCO Internals

When both exits are provided, OCO creates an OCO node with two SINGLE children (take-profit LIMIT + stop-loss STOP). When only one exit is provided, it creates a plain SINGLE order (no OCO wrapper needed).

At least one of `--take-profit` or `--stop-loss` is required. Price validation ensures take-profit and stop-loss are on the correct side (SELL: TP > SL; BUY: TP < SL).

## Multi-Leg Option Strategies

Named strategy builders that encode all the domain knowledge (instructions, order type, OCC symbols) so the LLM doesn't have to. Build offline, then place via spec mode.

### Covered Call (IMPORTANT: read carefully)

There are two different situations for covered calls:

**Selling a call against shares you ALREADY OWN** - this is just a single-leg option sell. Use the regular option command:

```bash
schwab-agent order place option --underlying F --expiration 2026-06-18 --strike 14 --call \
  --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 0.50 --confirm
```

**Buying shares AND selling a call in one atomic order** (new position) - use `covered-call`:

```bash
schwab-agent order build covered-call --underlying F --expiration 2026-06-18 --strike 14 \
  --quantity 1 --price 12.00 | schwab-agent order preview --spec -
```

- `--quantity 1` = 100 shares + 1 call contract (1 contract always = 100 shares)
- `--price` = net debit (what you pay total per share after subtracting call premium)
- Always NET_DEBIT, always BUY shares + SELL_TO_OPEN call
- Uses Schwab's `COVERED` complex order strategy type

### Vertical Spread

Two-leg spread (bull call, bear call, bull put, bear put). Order type auto-determined from strikes.

```bash
# Bull call spread (debit): buy lower strike, sell higher strike
schwab-agent order build vertical --underlying F --expiration 2026-06-18 \
  --long-strike 12 --short-strike 14 --call --open --quantity 1 --price 0.50

# Bear put spread (debit): buy higher strike, sell lower strike
schwab-agent order build vertical --underlying F --expiration 2026-06-18 \
  --long-strike 14 --short-strike 12 --put --open --quantity 1 --price 0.50
```

- `--long-strike` = the strike you're buying, `--short-strike` = the strike you're selling
- `--open` / `--close` controls TO_OPEN vs TO_CLOSE instructions
- NET_DEBIT or NET_CREDIT is auto-determined from strike relationship

### Iron Condor

Four-leg strategy. Strikes must be ordered: put-long < put-short < call-short < call-long.

```bash
schwab-agent order build iron-condor --underlying F --expiration 2026-06-18 \
  --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 \
  --open --quantity 1 --price 0.50
```

- Opening = NET_CREDIT (collecting premium), closing = NET_DEBIT

### Straddle

Two legs at the same strike (one call + one put).

```bash
# Long straddle (buy both)
schwab-agent order build straddle --underlying F --expiration 2026-06-18 \
  --strike 12 --buy --open --quantity 1 --price 1.50

# Short straddle (sell both)
schwab-agent order build straddle --underlying F --expiration 2026-06-18 \
  --strike 12 --sell --open --quantity 1 --price 1.50
```

### Strangle

Two legs at different strikes (one call + one put).

```bash
schwab-agent order build strangle --underlying F --expiration 2026-06-18 \
  --call-strike 14 --put-strike 10 --buy --open --quantity 1 --price 0.50
```

### Placing Multi-Leg Orders

All multi-leg builders output raw JSON. To place, pipe through spec mode:

```bash
schwab-agent order build vertical ... | schwab-agent order place --spec - --confirm
```

To preview first (recommended):

```bash
schwab-agent order build vertical ... | schwab-agent order preview --spec -
```

## Flags Reference

| Flag | Description | Used with |
|------|-------------|-----------|
| `--symbol` | Stock ticker | equity, bracket |
| `--action` | BUY, SELL, BUY_TO_OPEN, SELL_TO_CLOSE, etc. | all |
| `--quantity` | Number of shares or contracts | all |
| `--type` | MARKET, LIMIT, STOP, STOP_LIMIT | equity, bracket |
| `--price` | Limit price | LIMIT, STOP_LIMIT |
| `--stop-price` | Stop trigger price | STOP, STOP_LIMIT |
| `--take-profit` | Take-profit price (at least one exit required) | bracket, oco |
| `--stop-loss` | Stop-loss price (at least one exit required) | bracket, oco |
| `--underlying` | Underlying symbol | option, multi-leg strategies |
| `--expiration` | Option expiration (YYYY-MM-DD) | option, multi-leg strategies |
| `--strike` | Strike price | option, straddle, covered-call |
| `--call` / `--put` | Contract type | option, vertical |
| `--buy` / `--sell` | Direction | straddle, strangle |
| `--open` / `--close` | Opening or closing position | vertical, iron-condor, straddle, strangle |
| `--long-strike` / `--short-strike` | Strikes for legs being bought/sold | vertical |
| `--call-strike` / `--put-strike` | Separate strikes per leg | strangle, iron-condor |
| `--confirm` | Actually execute (omit to preview) | all mutable |
| `--account` | Account hash (uses default if set) | all |
| `--session` | NORMAL, AM, PM, SEAMLESS | all |
| `--duration` | DAY, GTC, FOK | all |

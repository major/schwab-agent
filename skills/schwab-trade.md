# Trading and Order Management

Place, preview, cancel, and replace orders. For order construction details (bracket, OCO, multi-leg strategies, flags), see `schwab-order-builder.md`.

## Safety Workflow

Follow this sequence before placing any real order:

1. Check current price: `schwab-agent quote get <symbol>`
2. Build the order JSON: `schwab-agent order build equity --symbol <SYM> --action BUY --quantity 10 --type LIMIT --price 200`
3. Preview (pipe build output): `schwab-agent order build equity --symbol <SYM> --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order preview --spec -`
4. Place with confirmation: `schwab-agent order place equity --symbol <SYM> --action BUY --quantity 10 --type LIMIT --price 200 --confirm`
5. Verify status: `schwab-agent order list --status WORKING`

## Safety Requirements

- Config flag: `"i-also-like-to-live-dangerously": true` must be set in config
- CLI flag: `--confirm` must be passed on every mutable command
- Both are required. Without both, the command will fail with a validation error.

## Intent Mapping

| User says | Order type | Command |
|-----------|-----------|---------|
| "buy at current price" | MARKET | `order place equity --type MARKET` (or omit --type) |
| "buy at $X or less" | LIMIT | `order place equity --type LIMIT --price X` |
| "sell if it drops to $X" | STOP | `order place equity --type STOP --stop-price X` |
| "sell at $X but not below $Y" | STOP_LIMIT | `order place equity --type STOP_LIMIT --stop-price X --price Y` |
| "buy with protection" | BRACKET (full) | `order place bracket --take-profit X --stop-loss Y` |
| "buy with a safety net" | BRACKET (stop only) | `order place bracket --stop-loss Y` |
| "buy with a profit target" | BRACKET (TP only) | `order place bracket --take-profit X` |
| "set take-profit and stop-loss" (already own shares) | OCO | `order place oco --action SELL --take-profit X --stop-loss Y` |
| "protect my position" (existing shares) | OCO (stop only) | `order place oco --action SELL --stop-loss Y` |
| "set a profit target" (existing shares) | OCO (TP only) | `order place oco --action SELL --take-profit X` |
| "buy calls/puts" | OPTION | `order place option --underlying SYM --call/--put ...` |
| "sell covered calls" (already own shares) | OPTION (single leg) | `order place option --action SELL_TO_OPEN --call ...` |
| "buy stock and sell a call" (new position) | COVERED | `order build covered-call ...` then place via spec |
| "bull call spread" | VERTICAL | `order build vertical --call --open --long-strike X --short-strike Y` |
| "iron condor" | IRON_CONDOR | `order build iron-condor --open ...` |
| "buy a straddle" | STRADDLE | `order build straddle --buy --open ...` |
| "buy a strangle" | STRANGLE | `order build strangle --buy --open ...` |

## Scenario Recipes

### Buy at market price

```bash
schwab-agent order place equity --symbol AAPL --action BUY --quantity 10 --confirm
```

### Buy with a limit price

```bash
schwab-agent order place equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --confirm
```

### Sell with a stop

```bash
schwab-agent order place equity --symbol TSLA --action SELL --quantity 5 --type STOP --stop-price 150 --confirm
```

### Buy options

```bash
schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --confirm
```

### Preview before placing

Preview requires `--spec` with JSON input. Use `order build` to generate the JSON, then pipe it:

```bash
schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order preview --spec -
```

### Cancel an order

```bash
schwab-agent order cancel 12345 --confirm
```

### Replace an order

Replace cancels the original order and creates a new one with a different ID. Only equity orders can be replaced.

```bash
schwab-agent order replace <order-id> --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 205 --confirm
# Original order → status: REPLACED. Use `order list --status WORKING` to find the new order ID.
```

## Listing and Getting Orders

```bash
schwab-agent order list
schwab-agent order list --status WORKING
schwab-agent order get <order-id>
```

## Spec Mode

Pass a raw JSON order payload directly instead of using flags:

```bash
# Inline JSON
schwab-agent order place --spec '{"orderType": "LIMIT", ...}' --confirm

# From file
schwab-agent order place --spec @order.json --confirm

# From stdin
cat order.json | schwab-agent order place --spec - --confirm
```

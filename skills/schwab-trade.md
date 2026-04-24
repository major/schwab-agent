# Trading and Order Management

Place, preview, cancel, replace, and repeat orders. For order construction details (multi-leg strategies, builder flags), see `schwab-order-builder.md`.

## Safety Requirements

Both are required for any mutable operation (place/cancel/replace):

1. Config: `"i-also-like-to-live-dangerously": true` in config file
2. CLI: `--confirm` flag on the command

## Safety Workflow

1. Check price: `schwab-agent quote get <symbol>`
2. Build order JSON: `schwab-agent order build equity --symbol SYM --action BUY --quantity 10 --type LIMIT --price 200`
3. Preview: `schwab-agent order build equity ... | schwab-agent order preview --spec -`
4. Place: `schwab-agent order place equity --symbol SYM --action BUY --quantity 10 --type LIMIT --price 200 --confirm`
5. Verify: `schwab-agent order list --status WORKING`

## Intent Mapping

| User says | Command |
|-----------|---------|
| "buy at current price" | `order place equity --action BUY --type MARKET` |
| "buy at $X or less" | `order place equity --action BUY --type LIMIT --price X` |
| "sell if it drops to $X" | `order place equity --action SELL --type STOP --stop-price X` |
| "sell at $X but not below $Y" | `order place equity --action SELL --type STOP_LIMIT --stop-price X --price Y` |
| "trailing stop" | `order place equity --action SELL --type TRAILING_STOP --stop-offset 2.50` |
| "buy with protection" | `order place bracket --action BUY --take-profit X --stop-loss Y` |
| "buy with a safety net" | `order place bracket --action BUY --stop-loss Y` |
| "set take-profit and stop-loss" (own shares) | `order place oco --action SELL --take-profit X --stop-loss Y` |
| "protect my position" (own shares) | `order place oco --action SELL --stop-loss Y` |
| "buy calls/puts" | `order place option --underlying SYM --call/--put --action BUY_TO_OPEN` |
| "sell covered calls" (own shares) | `order place option --action SELL_TO_OPEN --call ...` |
| "buy stock and sell a call" (new) | `order build covered-call ...` then place via spec |
| "bull call spread" | `order build vertical --call --open --long-strike X --short-strike Y` |
| "iron condor" | `order build iron-condor --open ...` |
| "straddle" | `order build straddle --buy --open ...` |
| "strangle" | `order build strangle --buy --open ...` |
| "calendar spread" | `order build calendar --call --open ...` |
| "diagonal spread" | `order build diagonal --call --open ...` |
| "collar" | `order build collar --open ...` |
| "repeat a previous order" | `order repeat <order-id>` |
| "re-place that order" | `order repeat <order-id> --confirm` |

## Listing and Getting Orders

```bash
schwab-agent order list                                    # Non-terminal orders only (default)
schwab-agent order list --status all                       # All orders including filled/canceled
schwab-agent order list --status WORKING --from 2025-01-01 --to 2025-01-31
schwab-agent order list --status WORKING --status PENDING_ACTIVATION
schwab-agent order list --status WORKING,FILLED,EXPIRED
schwab-agent order get <order-id>
```

Flags: `--status` (filter by status, repeatable), `--from`/`--to` (filter by entered time).

By default, `order list` filters out terminal statuses (FILLED, CANCELED, REJECTED, EXPIRED, REPLACED) to show only actionable orders. Use `--status all` to see everything, or pass explicit statuses to override the default.

The `--status` flag can be repeated or comma-separated to query multiple statuses. The Schwab API only accepts one status per request, so multiple statuses automatically fan out into separate API calls with merged, deduplicated results.

## Duration Aliases

Standard trading abbreviations are accepted for `--duration`:

| Alias | Expands to |
|-------|------------|
| GTC | GOOD_TILL_CANCEL |
| FOK | FILL_OR_KILL |
| IOC | IMMEDIATE_OR_CANCEL |

`DAY` is already short, so no alias needed. Case-insensitive.

## Placing Orders

```bash
# Equity
schwab-agent order place equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --confirm

# Option
schwab-agent order place option --underlying AAPL --expiration 2025-06-20 --strike 200 --call --action BUY_TO_OPEN --quantity 1 --confirm

# Bracket (entry + automatic exits)
schwab-agent order place bracket --symbol NVDA --action BUY --quantity 10 --type MARKET --take-profit 150 --stop-loss 120 --confirm

# OCO (exit orders for existing position)
schwab-agent order place oco --symbol AAPL --action SELL --quantity 100 --take-profit 160 --stop-loss 140 --confirm
```

## Preview

Preview requires `--spec` with JSON input. Pipe from `order build`:

```bash
schwab-agent order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 | schwab-agent order preview --spec -
```

## Cancel and Replace

```bash
schwab-agent order cancel <order-id> --confirm
schwab-agent order replace <order-id> --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 205 --confirm
```

Replace cancels the original and creates a new order (new ID). Only equity order flags accepted. Original order status becomes REPLACED.

## Repeat

Fetch an existing order, convert it to a submittable request, and optionally place it:

```bash
schwab-agent order repeat <order-id>            # Output reconstructed order JSON (default, same as --build)
schwab-agent order repeat <order-id> --preview   # Preview without placing
schwab-agent order repeat <order-id> --confirm   # Place the order (requires safety guards)
```

Only one mode flag allowed. `--build`, `--preview`, and `--confirm` are mutually exclusive.

## Spec Mode

Pass raw JSON order payload directly instead of using flags:

```bash
schwab-agent order place --spec '{"orderType": "LIMIT", ...}' --confirm
schwab-agent order place --spec @order.json --confirm
cat order.json | schwab-agent order place --spec - --confirm
```

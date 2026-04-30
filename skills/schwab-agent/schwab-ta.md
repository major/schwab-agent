# Technical Analysis Indicators

Eleven indicators computed locally from Schwab price history. Pattern: `schwab-agent ta <indicator> SYMBOL [flags]`.

## Shared Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--interval` | daily | daily, weekly, 1min, 5min, 15min, 30min |
| `--points` | 1 | Limit output to N most recent values (0 = all) |

Most indicators also accept `--period` (lookback window). For SMA, EMA, and RSI, `--period` may be repeated or comma-separated to calculate multiple lookbacks in one command, for example `--period 21,50,200`.

## Standard Indicators

All use `--period` for lookback window. Output is a time series with `datetime` plus indicator-specific keys. SMA, EMA, and RSI support multiple periods in one run.

| Indicator | Command | Period Default | Output Keys | Interpretation |
|-----------|---------|---------------|-------------|----------------|
| SMA | `ta sma` | 20 | `sma` | Simple moving average |
| EMA | `ta ema` | 20 | `ema` | Exponential moving average |
| RSI | `ta rsi` | 14 | `rsi` | 0-100. >70 overbought, <30 oversold |
| ATR | `ta atr` | 14 | `atr` | Volatility in price units. Higher = wider swings |
| ADX | `ta adx` | 14 | `adx`, `plus_di`, `minus_di` | >25 trending, <20 ranging. plus/minus_di show directional bias |

Examples:

```bash
schwab-agent ta rsi AAPL --period 9 --interval 5min --points 20
schwab-agent ta sma AAPL --period 21,50,200 --points 1
schwab-agent ta ema AAPL --period 12 --period 26 --points 5
```

## MACD - Moving Average Convergence/Divergence

Uses `--fast`/`--slow`/`--signal` instead of `--period`.

```bash
schwab-agent ta macd AAPL --fast 12 --slow 26 --signal 9 --points 10
```

| Flag | Default |
|------|---------|
| `--fast` | 12 |
| `--slow` | 26 |
| `--signal` | 9 |

Output keys: `macd`, `signal`, `histogram` (histogram = macd - signal).

## BBands - Bollinger Bands

```bash
schwab-agent ta bbands AAPL --period 20 --std-dev 2.5 --points 10
```

`--period` default: 20. `--std-dev` default: 2.0. Output keys: `upper`, `middle`, `lower`. Price near upper = relatively high, near lower = relatively low.

## Stoch - Stochastic Oscillator

```bash
schwab-agent ta stoch AAPL --k-period 14 --smooth-k 3 --d-period 3 --points 10
```

| Flag | Default |
|------|---------|
| `--k-period` | 14 |
| `--smooth-k` | 3 |
| `--d-period` | 3 |

Output keys: `slowk`, `slowd`. Both 0-100. Above 80 / below 20 are common signal thresholds.

## VWAP - Volume Weighted Average Price

Cumulative indicator, no `--period` flag. Only `--interval` and `--points`.

```bash
schwab-agent ta vwap AAPL --interval 5min --points 20
```

Output key: `vwap`.

## HV - Historical Volatility

Scalar summary (not time series). Includes regime classification.

```bash
schwab-agent ta hv AAPL --period 20 --interval daily
```

`--period` default: 20. Output keys: `daily_vol`, `weekly_vol`, `monthly_vol`, `annualized_vol`, `percentile_rank`, `regime` (low/normal/high/extreme), `min_vol`, `max_vol`, `mean_vol`.

## Expected Move - ATM Straddle Pricing

Scalar summary (not time series). Fetches ATM options from the chain and computes expected price range.

```bash
schwab-agent ta expected-move AAPL --dte 45
```

`--dte` default: 30 (target days to expiration). Output keys: `underlying_price`, `expiration`, `dte`, `straddle_price`, `expected_move`, `adjusted_move`, `upper_1x`, `lower_1x`, `upper_2x`, `lower_2x`.

## Output Format

Time series indicators return:

```json
{
  "data": {
    "indicator": "sma",
    "symbol": "AAPL",
    "interval": "daily",
    "period": 20,
    "values": [{"datetime": "2026-04-18", "sma": 198.42}]
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

SMA, EMA, and RSI with multiple periods return `periods` and key each available value by period. Early rows may omit longer-period keys until enough history exists; the latest rows include all requested periods when Schwab returns enough candles:

```json
{
  "data": {
    "indicator": "sma",
    "symbol": "AAPL",
    "interval": "daily",
    "periods": [21, 50, 200],
    "values": [{"datetime": "2026-04-18", "sma_21": 203.14, "sma_50": 198.42, "sma_200": 184.77}]
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

Scalar indicators (HV, Expected Move) return fields directly in `data` instead of a `values` array.

## Recipes

| Intent | Command |
|--------|---------|
| "Is AAPL overbought?" | `ta rsi AAPL` |
| "20-day moving average" | `ta sma AAPL` |
| "21/50/200-day SMA" | `ta sma AAPL --period 21,50,200 --points 1` |
| "50/200-day SMA crossover" | `ta sma AAPL --period 50,200 --points 5` |
| "MACD signal" | `ta macd AAPL --points 5` |
| "How volatile is AAPL?" | `ta atr AAPL` |
| "Near Bollinger Band?" | `ta bbands AAPL` |
| "Stochastic reading" | `ta stoch AAPL` |
| "Trending or ranging?" | `ta adx AAPL` |
| "EMA crossover (12/26)" | `ta ema AAPL --period 12,26 --points 5` |
| "Intraday RSI (5min)" | `ta rsi AAPL --interval 5min --points 20` |
| "Weekly MACD" | `ta macd NVDA --interval weekly --points 10` |
| "Tight Bollinger Bands" | `ta bbands AAPL --std-dev 1.5 --points 10` |
| "VWAP intraday" | `ta vwap AAPL --interval 5min --points 20` |
| "Volatility regime" | `ta hv AAPL` |
| "Expected move (30 days)" | `ta expected-move AAPL` |
| "Expected move (45 days)" | `ta expected-move AAPL --dte 45` |

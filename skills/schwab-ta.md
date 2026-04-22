# Technical Analysis Indicators

Eleven indicators computed locally from Schwab price history. Pattern: `schwab-agent ta <indicator> SYMBOL [flags]`.

## Shared Flags

All indicators accept these flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--interval` | daily | daily, weekly, 1min, 5min, 15min, 30min |
| `--points` | 0 (all) | Limit output to N most recent values |

Most indicators also accept `--period` (lookback window). Exceptions noted below.

## Indicators

### SMA - Simple Moving Average

```bash
schwab-agent ta sma AAPL
schwab-agent ta sma AAPL --period 200 --interval daily --points 10
```

`--period` default: 20

### EMA - Exponential Moving Average

```bash
schwab-agent ta ema AAPL
schwab-agent ta ema AAPL --period 26 --interval daily --points 5
```

`--period` default: 20

### RSI - Relative Strength Index

Values 0-100. Above 70 = conventionally overbought, below 30 = oversold.

```bash
schwab-agent ta rsi AAPL
schwab-agent ta rsi AAPL --period 9 --interval daily --points 20
```

`--period` default: 14

### ATR - Average True Range

Measures volatility in price units (not percent). Higher ATR = wider daily swings.

```bash
schwab-agent ta atr AAPL
schwab-agent ta atr AAPL --period 7 --interval daily --points 5
```

`--period` default: 14

### ADX - Average Directional Index

Trend strength (not direction). `plus_di`/`minus_di` indicate directional bias. Above 25 = trending, below 20 = ranging.

```bash
schwab-agent ta adx AAPL
schwab-agent ta adx AAPL --period 14 --interval daily --points 10
```

`--period` default: 14. Output includes `adx`, `plus_di`, `minus_di`.

### MACD - Moving Average Convergence/Divergence

Uses `--fast`/`--slow`/`--signal` instead of `--period`. Histogram = macd - signal.

```bash
schwab-agent ta macd AAPL
schwab-agent ta macd AAPL --fast 12 --slow 26 --signal 9 --points 10
```

| Flag | Default |
|------|---------|
| `--fast` | 12 |
| `--slow` | 26 |
| `--signal` | 9 |

Output includes `macd`, `signal`, `histogram`.

### BBands - Bollinger Bands

Upper, middle (SMA), lower bands. Price near upper = relatively high, near lower = relatively low.

```bash
schwab-agent ta bbands AAPL
schwab-agent ta bbands AAPL --period 20 --std-dev 2.5 --points 10
```

`--period` default: 20. `--std-dev` default: 2.0. Output includes `upper`, `middle`, `lower`.

### Stoch - Stochastic Oscillator

Slow %K and %D, both 0-100. Crossovers and extremes (above 80 / below 20) are common signals.

```bash
schwab-agent ta stoch AAPL
schwab-agent ta stoch AAPL --k-period 14 --smooth-k 3 --d-period 3 --points 10
```

| Flag | Default |
|------|---------|
| `--k-period` | 14 |
| `--smooth-k` | 3 |
| `--d-period` | 3 |

Output includes `slowk`, `slowd`.

### VWAP - Volume Weighted Average Price

Cumulative indicator - no `--period` flag. Only `--interval` and `--points`.

```bash
schwab-agent ta vwap AAPL
schwab-agent ta vwap AAPL --interval 5min --points 20
```

Output includes `vwap`.

### HV - Historical Volatility

Returns a scalar summary (not a time series). Includes regime classification.

```bash
schwab-agent ta hv AAPL
schwab-agent ta hv AAPL --period 20 --interval daily
```

`--period` default: 20. Output fields: `daily_vol`, `weekly_vol`, `monthly_vol`, `annualized_vol`, `percentile_rank`, `regime` (low/normal/high/extreme), `min_vol`, `max_vol`, `mean_vol`.

### Expected Move - ATM Straddle Pricing

Returns a scalar summary (not a time series). Fetches ATM options from the chain and computes the expected price range.

```bash
schwab-agent ta expected-move AAPL
schwab-agent ta expected-move AAPL --dte 45
```

`--dte` default: 30 (target days to expiration for option selection). Output fields: `underlying_price`, `expiration`, `dte`, `straddle_price`, `expected_move`, `adjusted_move`, `upper_1x`, `lower_1x`, `upper_2x`, `lower_2x`.

## Output Format

All indicators return the same envelope. The `values` array contains objects with `datetime` plus indicator-specific fields.

Single-value example (SMA, EMA, RSI, ATR):

```json
{
  "data": {
    "indicator": "sma",
    "symbol": "AAPL",
    "interval": "daily",
    "period": 20,
    "values": [
      {"datetime": "2026-04-18", "sma": 198.42},
      {"datetime": "2026-04-21", "sma": 199.10}
    ]
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

Multi-value examples - value keys by indicator:

| Indicator | Value keys |
|-----------|------------|
| ADX | `adx`, `plus_di`, `minus_di` |
| MACD | `macd`, `signal`, `histogram` |
| BBands | `upper`, `middle`, `lower` |
| Stoch | `slowk`, `slowd` |

Scalar output examples (HV, Expected Move):

```json
{
  "data": {
    "indicator": "hv",
    "symbol": "AAPL",
    "period": 20,
    "daily_vol": 1.23,
    "weekly_vol": 2.75,
    "monthly_vol": 5.62,
    "annualized_vol": 19.54,
    "percentile_rank": 42.0,
    "regime": "normal",
    "min_vol": 12.10,
    "max_vol": 38.90,
    "mean_vol": 21.30
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

```json
{
  "data": {
    "indicator": "expected-move",
    "symbol": "AAPL",
    "underlying_price": 195.50,
    "expiration": "2026-05-16",
    "dte": 24,
    "straddle_price": 9.80,
    "expected_move": 9.80,
    "adjusted_move": 8.33,
    "upper_1x": 203.83,
    "lower_1x": 187.17,
    "upper_2x": 212.16,
    "lower_2x": 178.84
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

## Recipes

| Intent | Command |
|--------|---------|
| "Is AAPL overbought?" | `schwab-agent ta rsi AAPL --points 1` |
| "20-day moving average for AAPL" | `schwab-agent ta sma AAPL --period 20 --points 1` |
| "50/200-day SMA crossover check" | `schwab-agent ta sma AAPL --period 50 --points 5` then `--period 200 --points 5` |
| "MACD signal for AAPL" | `schwab-agent ta macd AAPL --points 5` |
| "How volatile is AAPL?" | `schwab-agent ta atr AAPL --points 1` |
| "Is AAPL near its Bollinger Band?" | `schwab-agent ta bbands AAPL --points 1` |
| "Stochastic reading for AAPL" | `schwab-agent ta stoch AAPL --points 1` |
| "Is AAPL trending or ranging?" | `schwab-agent ta adx AAPL --points 1` |
| "EMA crossover (12/26) for AAPL" | `schwab-agent ta ema AAPL --period 12 --points 5` then `--period 26 --points 5` |
| "Intraday RSI on 5-minute bars" | `schwab-agent ta rsi AAPL --interval 5min --points 20` |
| "Weekly MACD for NVDA" | `schwab-agent ta macd NVDA --interval weekly --points 10` |
| "Tighten Bollinger Bands to 1.5 std dev" | `schwab-agent ta bbands AAPL --std-dev 1.5 --points 10` |
| "VWAP for AAPL intraday" | `schwab-agent ta vwap AAPL --interval 5min --points 20` |
| "Historical volatility regime for AAPL" | `schwab-agent ta hv AAPL` |
| "Expected move for AAPL next 30 days" | `schwab-agent ta expected-move AAPL` |
| "Expected move for AAPL next 45 days" | `schwab-agent ta expected-move AAPL --dte 45` |

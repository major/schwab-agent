# Technical Analysis Indicators

Eight technical analysis indicators computed from Schwab price history. All commands follow the pattern `schwab-agent ta <indicator> <SYMBOL>` and return JSON. Indicators are calculated locally from candle data - no third-party TA service involved.

All intervals: `daily` (default), `weekly`, `1min`, `5min`, `15min`, `30min`.

Use `--points N` on any indicator to limit output to the N most recent values. Omit it (or pass `0`) to get all available values.

## SMA - Simple Moving Average

```bash
schwab-agent ta sma AAPL
schwab-agent ta sma AAPL --period 50
schwab-agent ta sma AAPL --period 200 --interval daily
schwab-agent ta sma AAPL --period 20 --points 10
```

| Flag | Default | Description |
|------|---------|-------------|
| `--period` | 20 | Lookback period |
| `--interval` | daily | Data interval |
| `--points` | 0 (all) | Limit output to N most recent values |

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

## EMA - Exponential Moving Average

```bash
schwab-agent ta ema AAPL
schwab-agent ta ema AAPL --period 12
schwab-agent ta ema AAPL --period 26 --interval daily --points 5
```

| Flag | Default | Description |
|------|---------|-------------|
| `--period` | 20 | Lookback period |
| `--interval` | daily | Data interval |
| `--points` | 0 (all) | Limit output to N most recent values |

```json
{
  "data": {
    "indicator": "ema",
    "symbol": "AAPL",
    "interval": "daily",
    "period": 20,
    "values": [
      {"datetime": "2026-04-18", "ema": 197.85},
      {"datetime": "2026-04-21", "ema": 198.63}
    ]
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

## RSI - Relative Strength Index

RSI oscillates between 0 and 100. Values above 70 are conventionally overbought; below 30 are oversold. The tool returns the values - you decide what to do with them.

```bash
schwab-agent ta rsi AAPL
schwab-agent ta rsi AAPL --period 14
schwab-agent ta rsi AAPL --period 9 --interval daily --points 20
```

| Flag | Default | Description |
|------|---------|-------------|
| `--period` | 14 | Lookback period |
| `--interval` | daily | Data interval |
| `--points` | 0 (all) | Limit output to N most recent values |

```json
{
  "data": {
    "indicator": "rsi",
    "symbol": "AAPL",
    "interval": "daily",
    "period": 14,
    "values": [
      {"datetime": "2026-04-18", "rsi": 58.34},
      {"datetime": "2026-04-21", "rsi": 61.02}
    ]
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

## MACD - Moving Average Convergence/Divergence

MACD has its own flag set (fast/slow/signal) instead of the shared `--period` flag. The histogram is `macd - signal`.

```bash
schwab-agent ta macd AAPL
schwab-agent ta macd AAPL --fast 12 --slow 26 --signal 9
schwab-agent ta macd AAPL --interval daily --points 10
```

| Flag | Default | Description |
|------|---------|-------------|
| `--fast` | 12 | Fast EMA period |
| `--slow` | 26 | Slow EMA period |
| `--signal` | 9 | Signal EMA period |
| `--interval` | daily | Data interval |
| `--points` | 0 (all) | Limit output to N most recent values |

```json
{
  "data": {
    "indicator": "macd",
    "symbol": "AAPL",
    "interval": "daily",
    "fast": 12,
    "slow": 26,
    "signal": 9,
    "values": [
      {"datetime": "2026-04-18", "macd": 1.23, "signal": 0.98, "histogram": 0.25},
      {"datetime": "2026-04-21", "macd": 1.45, "signal": 1.08, "histogram": 0.37}
    ]
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

## ATR - Average True Range

ATR measures volatility in price units (not percent). Higher ATR means wider daily swings.

```bash
schwab-agent ta atr AAPL
schwab-agent ta atr AAPL --period 14
schwab-agent ta atr AAPL --period 7 --interval daily --points 5
```

| Flag | Default | Description |
|------|---------|-------------|
| `--period` | 14 | Lookback period |
| `--interval` | daily | Data interval |
| `--points` | 0 (all) | Limit output to N most recent values |

```json
{
  "data": {
    "indicator": "atr",
    "symbol": "AAPL",
    "interval": "daily",
    "period": 14,
    "values": [
      {"datetime": "2026-04-18", "atr": 3.87},
      {"datetime": "2026-04-21", "atr": 4.12}
    ]
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

## BBands - Bollinger Bands

Three bands: upper, middle (SMA), and lower. Price touching the upper band is relatively high; touching the lower band is relatively low. The `std_dev` field in the output reflects the `--std-dev` flag used.

```bash
schwab-agent ta bbands AAPL
schwab-agent ta bbands AAPL --period 20 --std-dev 2.0
schwab-agent ta bbands AAPL --period 20 --std-dev 2.5 --points 10
```

| Flag | Default | Description |
|------|---------|-------------|
| `--period` | 20 | Lookback period |
| `--std-dev` | 2.0 | Standard deviations for band width |
| `--interval` | daily | Data interval |
| `--points` | 0 (all) | Limit output to N most recent values |

```json
{
  "data": {
    "indicator": "bbands",
    "symbol": "AAPL",
    "interval": "daily",
    "period": 20,
    "std_dev": 2.0,
    "values": [
      {"datetime": "2026-04-18", "upper": 210.45, "middle": 198.42, "lower": 186.39},
      {"datetime": "2026-04-21", "upper": 211.20, "middle": 199.10, "lower": 187.00}
    ]
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

## Stoch - Stochastic Oscillator

Returns slow %K and slow %D. Both oscillate between 0 and 100. Crossovers and extreme readings (above 80 / below 20) are common signals. The output keys are `slowk` and `slowd`.

```bash
schwab-agent ta stoch AAPL
schwab-agent ta stoch AAPL --k-period 14 --smooth-k 3 --d-period 3
schwab-agent ta stoch AAPL --interval daily --points 10
```

| Flag | Default | Description |
|------|---------|-------------|
| `--k-period` | 14 | Fast %K lookback period |
| `--smooth-k` | 3 | Slow %K smoothing period |
| `--d-period` | 3 | Slow %D period |
| `--interval` | daily | Data interval |
| `--points` | 0 (all) | Limit output to N most recent values |

```json
{
  "data": {
    "indicator": "stoch",
    "symbol": "AAPL",
    "interval": "daily",
    "k_period": 14,
    "smooth_k": 3,
    "d_period": 3,
    "values": [
      {"datetime": "2026-04-18", "slowk": 72.14, "slowd": 68.90},
      {"datetime": "2026-04-21", "slowk": 75.33, "slowd": 71.52}
    ]
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

## ADX - Average Directional Index

ADX measures trend strength (not direction). `plus_di` and `minus_di` indicate directional bias. ADX above 25 generally signals a trending market; below 20 suggests ranging.

```bash
schwab-agent ta adx AAPL
schwab-agent ta adx AAPL --period 14
schwab-agent ta adx AAPL --period 14 --interval daily --points 10
```

| Flag | Default | Description |
|------|---------|-------------|
| `--period` | 14 | Lookback period |
| `--interval` | daily | Data interval |
| `--points` | 0 (all) | Limit output to N most recent values |

```json
{
  "data": {
    "indicator": "adx",
    "symbol": "AAPL",
    "interval": "daily",
    "period": 14,
    "values": [
      {"datetime": "2026-04-18", "adx": 28.41, "plus_di": 22.10, "minus_di": 14.33},
      {"datetime": "2026-04-21", "adx": 29.87, "plus_di": 23.45, "minus_di": 13.91}
    ]
  },
  "metadata": {"timestamp": "2026-04-22T10:00:00Z"}
}
```

## Recipes

| Intent | Command |
|--------|---------|
| "Is AAPL overbought?" | `schwab-agent ta rsi AAPL --points 1` |
| "20-day moving average for AAPL" | `schwab-agent ta sma AAPL --period 20 --points 1` |
| "50-day and 200-day SMA crossover check" | `schwab-agent ta sma AAPL --period 50 --points 5` then `--period 200 --points 5` |
| "MACD signal for AAPL" | `schwab-agent ta macd AAPL --points 5` |
| "How volatile is AAPL right now?" | `schwab-agent ta atr AAPL --points 1` |
| "Is AAPL near its Bollinger Band?" | `schwab-agent ta bbands AAPL --points 1` |
| "Stochastic reading for AAPL" | `schwab-agent ta stoch AAPL --points 1` |
| "Is AAPL trending or ranging?" | `schwab-agent ta adx AAPL --points 1` |
| "EMA crossover (12/26) for AAPL" | `schwab-agent ta ema AAPL --period 12 --points 5` then `--period 26 --points 5` |
| "Intraday RSI on 5-minute bars" | `schwab-agent ta rsi AAPL --interval 5min --points 20` |
| "Weekly MACD for NVDA" | `schwab-agent ta macd NVDA --interval weekly --points 10` |
| "Tighten Bollinger Bands to 1.5 std dev" | `schwab-agent ta bbands AAPL --std-dev 1.5 --points 10` |

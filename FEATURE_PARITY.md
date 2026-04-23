# Feature Parity: schwab-agent vs schwab-py

Gap analysis of what schwab-py supports that schwab-agent does not. Streaming/websocket features excluded (not relevant for LLM consumers).

Features are rated for LLM trading usefulness (how valuable is this for an AI agent making trading decisions?) and implementation difficulty.

> schwab-agent has features schwab-py does NOT: technical analysis indicators, schema introspection, and OCC symbol build/parse. Those are not listed here since they're already advantages.

## Usefulness Rating Scale

| Rating | Meaning |
|--------|---------|
| 5 | Essential for competent LLM trading |
| 4 | Very useful, covers common trading scenarios |
| 3 | Moderately useful, specific but real use cases |
| 2 | Niche, rarely needed for typical LLM workflows |
| 1 | Specialized, almost never relevant |

## Difficulty Rating Scale

| Rating | Meaning |
|--------|---------|
| Easy | Models/enums already exist, just wire up CLI flags or query params |
| Medium | Some builder logic needed, moderate testing |
| Hard | Significant new builder logic, validation, multi-leg coordination |

---

## Critical Gaps

### Account Positions (`?fields=positions`)

**Usefulness: 5** | **Difficulty: Easy**

The model layer is fully ready: `SecuritiesAccount` has `Positions []Position` with all fields (quantities, P&L, market value, instrument details). However, the client methods pass `nil` query params, so the Schwab API never includes position data in the response. The `Positions` slice always deserializes as empty.

schwab-py: `get_account(account_hash, fields=Client.Account.Fields.POSITIONS)`
schwab-agent: model exists, just needs `?fields=positions` query param passed to the API

Fix is minimal: add a `--positions` flag (or always include it) and pass `map[string]string{"fields": "positions"}` to `doGet`. No new model code needed.

This is probably the single highest-impact gap. An LLM cannot make informed trading decisions without knowing what it already owns.

### Trailing Stop Orders

**Usefulness: 5** | **Difficulty: Medium**

`parseOrderType()` doesn't accept TRAILING_STOP or TRAILING_STOP_LIMIT. The models already have fields for `stopPriceLinkBasis`, `stopPriceLinkType`, `stopPriceOffset`, and `stopType`, but the order builder doesn't expose them.

Trailing stops are fundamental risk management. An LLM managing a portfolio should be able to set a trailing stop at X% or $Y below market price and let it ride.

Implementation requires:
- Add TRAILING_STOP, TRAILING_STOP_LIMIT to `parseOrderType()`
- Add CLI flags: `--stop-offset`, `--stop-link-basis`, `--stop-link-type`, `--stop-type`
- Wire flags into order builder
- Activation price flag for conditional trailing stops

### Special Instructions (ALL_OR_NONE, DO_NOT_REDUCE)

**Usefulness: 4** | **Difficulty: Easy**

No `--special-instruction` flag on order commands. The model field exists (`SpecialInstruction`) but isn't populated.

ALL_OR_NONE prevents partial fills, which matters when an LLM is sizing positions for specific portfolio allocations. DO_NOT_REDUCE prevents share count adjustment on dividends/splits.

---

## High-Value Gaps

### Quote Field Filtering

**Usefulness: 4** | **Difficulty: Easy**

Quote endpoints don't pass `fields` (QUOTE, FUNDAMENTAL, EXTENDED, REFERENCE, REGULAR) or `indicative` params. LLMs get the default field set with no way to request fundamental data (P/E, dividend yield, etc.) alongside price data.

An LLM doing any kind of valuation-aware trading needs FUNDAMENTAL fields. Currently requires a separate instrument lookup.

### Stop Type (STANDARD, BID, ASK, LAST, MARK)

**Usefulness: 4** | **Difficulty: Easy**

No `--stop-type` flag. Defaults to whatever Schwab picks. For options and volatile stocks, an LLM should specify whether the stop triggers on BID, ASK, LAST, or MARK price. MARK is particularly important for options since it reflects fair value.

### Market on Close / Limit on Close

**Usefulness: 3** | **Difficulty: Easy**

MARKET_ON_CLOSE and LIMIT_ON_CLOSE not in `parseOrderType()`. Useful for end-of-day rebalancing strategies. An LLM running a daily rebalance model would use these to execute at the closing auction price.

### First-Triggers-Second Utility

**Usefulness: 3** | **Difficulty: Medium**

schwab-agent has bracket orders and OCO, but no general-purpose trigger mechanism. schwab-py's `first_triggers_second()` lets you chain any two orders: "buy this stock, then immediately place a covered call." An LLM could compose multi-step strategies without manual sequencing.

Currently the bracket builder hardcodes the trigger pattern. A general trigger utility would enable more creative order flows.

### Order Repeat/Reconstruction

**Usefulness: 3** | **Difficulty: Medium**

schwab-py's `construct_repeat_order()` takes a historical order and rebuilds it for resubmission (stripping filled quantities, resetting status, etc.). An LLM that identifies a previously successful trade pattern could repeat it without manually reconstructing every field.

The Python-specific `code_for_builder()` doesn't translate, but the repeat concept does.

---

## Medium-Value Gaps

### Destination Routing

**Usefulness: 2** | **Difficulty: Easy**

No `--destination` flag for exchange routing (INET, ECN_ARCA, CBOE, AMEX, etc.). The model field exists. Relevant for LLMs that care about execution quality or need to route to specific exchanges for liquidity reasons. Most LLM trading probably doesn't need this.

### Price Link Basis/Type

**Usefulness: 2** | **Difficulty: Easy**

`priceLinkBasis` (MANUAL, BASE, TRIGGER, LAST, BID, ASK, etc.) and `priceLinkType` (VALUE, PERCENT, TICK) fields exist in models but aren't exposed. These control how limit prices adjust relative to a reference price. Advanced feature, but useful for LLMs implementing dynamic pricing strategies.

### Calendar Spreads

**Usefulness: 2** | **Difficulty: Medium**

Not in the order builder. A calendar spread sells a near-term option and buys a longer-term option at the same strike. Useful for income strategies and volatility plays, but an LLM could construct this manually with two separate legs using the existing option builder.

### Diagonal Spreads

**Usefulness: 2** | **Difficulty: Medium**

Not in the order builder. Like a calendar but with different strikes. Same manual workaround as calendar spreads - two legs at different strikes and expirations.

### Collar / Collar with Stock

**Usefulness: 2** | **Difficulty: Medium**

Protective collar (long stock + long put + short call). Not in builder. Useful for portfolio protection, but an LLM could construct the legs individually. A dedicated builder would reduce errors and make intent clearer.

---

## Low-Value Gaps

### Butterfly Spreads

**Usefulness: 1** | **Difficulty: Hard**

Three-leg or four-leg strategy. Rarely used by automated systems. An LLM would need significant options expertise to deploy these profitably. Manual leg construction is viable.

### Back Ratio Spreads

**Usefulness: 1** | **Difficulty: Hard**

Unequal number of long and short options. Very specialized volatility play. Not something most LLM trading agents would attempt.

### Double Diagonal

**Usefulness: 1** | **Difficulty: Hard**

Four-leg strategy combining two diagonals. Extremely specialized. Manual construction viable for the rare case an LLM would use this.

### Unbalanced Strategies

**Usefulness: 1** | **Difficulty: Hard**

Unbalanced butterfly, condor, vertical roll variants. Highly specialized, almost never relevant for automated trading.

### Exercise Orders

**Usefulness: 1** | **Difficulty: Easy**

Early exercise of options. LLMs should almost never do this (selling the option captures remaining time value). Only relevant for deep ITM options near expiration with dividend capture.

### Cabinet Orders

**Usefulness: 1** | **Difficulty: Easy**

Closing worthless options for a nominal price ($0.01). Very niche, end-of-cycle cleanup.

### NON_MARKETABLE Order Type

**Usefulness: 1** | **Difficulty: Easy**

Limit orders priced away from the market. Schwab usually infers this from the limit price. Explicitly setting it adds little value.

---

## Summary by Priority

If implementing in order of LLM trading value per effort:

| Priority | Feature | Usefulness | Difficulty |
|----------|---------|------------|------------|
| 1 | Account positions field | 5 | Easy |
| 2 | Trailing stop orders | 5 | Medium |
| 3 | Special instructions | 4 | Easy |
| 4 | Quote field filtering | 4 | Easy |
| 5 | Stop type flag | 4 | Easy |
| 6 | Market/Limit on Close | 3 | Easy |
| 7 | First-triggers-second | 3 | Medium |
| 8 | Order repeat | 3 | Medium |
| 9 | Destination routing | 2 | Easy |
| 10 | Price link basis/type | 2 | Easy |
| 11 | Calendar spreads | 2 | Medium |
| 12 | Diagonal spreads | 2 | Medium |
| 13 | Collar builder | 2 | Medium |
| 14 | Butterfly builder | 1 | Hard |
| 15 | Back ratio builder | 1 | Hard |
| 16 | Double diagonal builder | 1 | Hard |
| 17 | Unbalanced strategies | 1 | Hard |
| 18 | Exercise orders | 1 | Easy |
| 19 | Cabinet orders | 1 | Easy |
| 20 | NON_MARKETABLE type | 1 | Easy |

The first 5 items would cover the most impactful gaps with reasonable effort. Items 1, 3, 4, and 5 are particularly low-hanging fruit since the model fields already exist.

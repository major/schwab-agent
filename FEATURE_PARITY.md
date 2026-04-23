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

### ~~Account Positions (`?fields=positions`)~~ ✅

**Usefulness: 5** | **Difficulty: Easy** | **Status: Implemented**

`--positions` flag on `account list` and `account get` passes `?fields=positions` to the API. Client methods accept variadic `fields ...string`.

### ~~Trailing Stop Orders~~ ✅

**Usefulness: 5** | **Difficulty: Medium** | **Status: Implemented**

TRAILING_STOP and TRAILING_STOP_LIMIT supported for equity orders via `--stop-offset`, `--stop-link-basis`, `--stop-link-type`, `--stop-type`, `--activation-price` flags. Defaults: link-basis=LAST, link-type=VALUE, stop-type=STANDARD. TRAILING_STOP_LIMIT uses `priceOffset` (not `price`) per Schwab API. Option orders reject trailing stop types at validation.

### ~~Special Instructions (ALL_OR_NONE, DO_NOT_REDUCE)~~ ✅

**Usefulness: 4** | **Difficulty: Easy** | **Status: Implemented**

`--special-instruction` flag on both equity and option order commands. Supports ALL_OR_NONE, DO_NOT_REDUCE, ALL_OR_NONE_DO_NOT_REDUCE.

---

## High-Value Gaps

### ~~Quote Field Filtering~~ ✅

**Usefulness: 4** | **Difficulty: Easy** | **Status: Implemented**

`--fields` (repeatable StringSliceFlag) and `--indicative` (BoolFlag) on `quote get`. Valid fields: quote, fundamental, extended, reference, regular (case-insensitive). LLMs can now request fundamental data (P/E, dividend yield, etc.) alongside price data without a separate instrument lookup.

### ~~Stop Type (STANDARD, BID, ASK, LAST, MARK)~~ ✅

**Usefulness: 4** | **Difficulty: Easy** | **Status: Implemented**

`--stop-type` flag with STANDARD, BID, ASK, LAST, MARK values on equity order commands. LLMs can now specify whether stops trigger on BID, ASK, LAST, or MARK price. MARK is particularly important for options since it reflects fair value.

### ~~Market on Close / Limit on Close~~ ✅

**Usefulness: 3** | **Difficulty: Easy** | **Status: Implemented**

`parseOrderType()` handles "MOC" -> MARKET_ON_CLOSE and "LOC" -> LIMIT_ON_CLOSE. LLMs can now use `order build equity --type MOC` or `--type MARKET_ON_CLOSE` for end-of-day rebalancing strategies and closing auction execution.

### ~~First-Triggers-Second Utility~~ ✅

**Usefulness: 3** | **Difficulty: Medium** | **Status: Implemented**

`order build fts --primary <spec> --secondary <spec>` command. Spec can be inline JSON, @file, or - for stdin. Outputs TRIGGER->SINGLE nested structure. LLMs can now compose multi-step strategies: "buy this stock, then immediately place a covered call" without manual sequencing or hardcoded bracket patterns.

### ~~Order Repeat/Reconstruction~~ ✅

**Usefulness: 3** | **Difficulty: Medium** | **Status: Implemented**

`order repeat <order-id>` with `--build` (default, raw JSON), `--preview` (preview endpoint), `--confirm` (place with safety guards). LLMs can now identify previously successful trade patterns and repeat them without manually reconstructing every field. Handles stripping filled quantities and resetting status automatically.

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

| Priority | Feature | Usefulness | Difficulty | Status |
|----------|---------|------------|------------|--------|
| ~~1~~ | ~~Account positions field~~ | ~~5~~ | ~~Easy~~ | ✅ |
| ~~2~~ | ~~Trailing stop orders~~ | ~~5~~ | ~~Medium~~ | ✅ |
| ~~3~~ | ~~Special instructions~~ | ~~4~~ | ~~Easy~~ | ✅ |
| ~~4~~ | ~~Quote field filtering~~ | ~~4~~ | ~~Easy~~ | ✅ |
| ~~5~~ | ~~Stop type flag~~ | ~~4~~ | ~~Easy~~ | ✅ |
| ~~6~~ | ~~Market/Limit on Close~~ | ~~3~~ | ~~Easy~~ | ✅ |
| ~~7~~ | ~~First-triggers-second~~ | ~~3~~ | ~~Medium~~ | ✅ |
| ~~8~~ | ~~Order repeat~~ | ~~3~~ | ~~Medium~~ | ✅ |
| 9 | Destination routing | 2 | Easy | |
| 10 | Price link basis/type | 2 | Easy | |
| 11 | Calendar spreads | 2 | Medium | |
| 12 | Diagonal spreads | 2 | Medium | |
| 13 | Collar builder | 2 | Medium | |
| 14 | Butterfly builder | 1 | Hard | |
| 15 | Back ratio builder | 1 | Hard | |
| 16 | Double diagonal builder | 1 | Hard | |
| 17 | Unbalanced strategies | 1 | Hard | |
| 18 | Exercise orders | 1 | Easy | |
| 19 | Cabinet orders | 1 | Easy | |
| 20 | NON_MARKETABLE type | 1 | Easy | |

Items 1-8 are implemented. All high-value gaps (usefulness 3-4, easy-medium difficulty) are now complete.

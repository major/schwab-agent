#!/bin/bash
# Smoke tests for schwab-agent CLI.
#
# Tier 1 (no auth): help text, symbol build/parse, all order build permutations.
#   Runs in CI and locally.
#
# Tier 2 (auth required): read-only API commands (account, quote, chain, etc.).
#   Runs locally only when a valid auth token exists.
#
# Usage:
#   SMOKE_TIER=1 ./scripts/smoke-test.sh   # Tier 1 only (CI)
#   SMOKE_TIER=2 ./scripts/smoke-test.sh   # Tier 2 only
#   ./scripts/smoke-test.sh                # Both tiers (default)
#
# Environment:
#   SMOKE_TIER  - "1", "2", or "all" (default: "all")
#   SMOKE_BIN   - Path to the binary (default: ./schwab-agent)

set -uo pipefail

BINARY="${SMOKE_BIN:-./schwab-agent}"
TIER="${SMOKE_TIER:-all}"
PASS=0
FAIL=0
SKIP=0

# Disable colors when stdout is not a terminal (CI logs, pipes).
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BOLD='\033[1m'
    NC='\033[0m'
else
    RED=''
    GREEN=''
    YELLOW=''
    BOLD=''
    NC=''
fi

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

# run_test validates that a command exits 0 and returns a JSON envelope with
# a "data" key (the standard output contract for most commands).
run_test() {
    local name="$1"
    shift
    local output
    if output=$("$BINARY" "$@" 2>&1); then
        if echo "$output" | jq -e '.data' > /dev/null 2>&1; then
            printf "${GREEN}PASS${NC} %s\n" "$name"
            PASS=$((PASS + 1))
        else
            printf "${RED}FAIL${NC} %s (no .data in JSON)\n" "$name"
            printf "  %s\n" "$output" | head -3
            FAIL=$((FAIL + 1))
        fi
    else
        local rc=$?
        printf "${RED}FAIL${NC} %s (exit %d)\n" "$name" "$rc"
        printf "  %s\n" "$output" | head -3
        FAIL=$((FAIL + 1))
    fi
}

# run_build_test validates that an order build command exits 0 and produces
# valid JSON. Order build output is raw JSON (no envelope).
run_build_test() {
    local name="$1"
    shift
    local output
    if output=$("$BINARY" "$@" 2>&1); then
        if echo "$output" | jq -e '.' > /dev/null 2>&1; then
            printf "${GREEN}PASS${NC} %s\n" "$name"
            PASS=$((PASS + 1))
        else
            printf "${RED}FAIL${NC} %s (invalid JSON)\n" "$name"
            printf "  %s\n" "$output" | head -3
            FAIL=$((FAIL + 1))
        fi
    else
        local rc=$?
        printf "${RED}FAIL${NC} %s (exit %d)\n" "$name" "$rc"
        printf "  %s\n" "$output" | head -3
        FAIL=$((FAIL + 1))
    fi
}

# run_help_test validates that --help exits 0 and produces output.
run_help_test() {
    local name="$1"
    shift
    if "$BINARY" "$@" --help > /dev/null 2>&1; then
        printf "${GREEN}PASS${NC} %s --help\n" "$name"
        PASS=$((PASS + 1))
    else
        printf "${RED}FAIL${NC} %s --help (exit %d)\n" "$name" "$?"
        FAIL=$((FAIL + 1))
    fi
}

# run_error_test validates that a command exits non-zero (expected error).
run_error_test() {
    local name="$1"
    shift
    if ! "$BINARY" "$@" > /dev/null 2>&1; then
        printf "${GREEN}PASS${NC} %s (expected error)\n" "$name"
        PASS=$((PASS + 1))
    else
        printf "${RED}FAIL${NC} %s (expected non-zero exit, got 0)\n" "$name"
        FAIL=$((FAIL + 1))
    fi
}

# run_dispatch_test validates that a shorthand command dispatches correctly.
# Dispatch succeeds if exit code is NOT 1 (validation error). Exit 0 (success with
# token) or exit 3 (auth error without token) both prove dispatch happened.
# Usage: run_dispatch_test "test name" command args...
run_dispatch_test() {
    local name="$1"
    shift
    "$BINARY" "$@" > /dev/null 2>&1
    local actual_code=$?
    if [ "$actual_code" -ne 1 ]; then
        printf "${GREEN}PASS${NC} %s (exit %d, dispatch verified)\n" "$name" "$actual_code"
        PASS=$((PASS + 1))
    else
        printf "${RED}FAIL${NC} %s (exit 1 = validation error, dispatch failed)\n" "$name"
        FAIL=$((FAIL + 1))
    fi
}

section() {
    printf "\n${BOLD}--- %s ---${NC}\n" "$1"
}

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

printf "${BOLD}Building schwab-agent...${NC}\n"
go build -o "$BINARY" ./cmd/schwab-agent/
printf "Binary: %s\n" "$("$BINARY" --version 2>/dev/null || echo 'unknown')"

# ===================================================================
# TIER 1: No auth required
# ===================================================================

if [ "$TIER" = "all" ] || [ "$TIER" = "1" ]; then

printf "\n${BOLD}=== TIER 1: No Auth Required ===${NC}\n"

# -------------------------------------------------------------------
# Help text for every command and subcommand
# -------------------------------------------------------------------

section "Help Text"

run_help_test "root"
run_help_test "auth"              auth
run_help_test "auth login"        auth login
run_help_test "auth status"       auth status
run_help_test "auth refresh"      auth refresh
run_help_test "account"           account
run_help_test "account summary"   account summary
run_help_test "account list"      account list
run_help_test "account get"       account get
run_help_test "account numbers"   account numbers
run_help_test "account set-default" account set-default
run_help_test "account transaction" account transaction
run_help_test "account transaction list" account transaction list
run_help_test "account transaction get"  account transaction get
run_help_test "position"          position
run_help_test "position list"     position list
run_help_test "quote"             quote
run_help_test "quote get"         quote get
run_help_test "order"             order
run_help_test "order list"        order list
run_help_test "order get"         order get
run_help_test "order place"       order place
run_help_test "order preview"     order preview
run_help_test "order cancel"      order cancel
run_help_test "order replace"     order replace
run_help_test "order build"       order build
run_help_test "order build equity"       order build equity
run_help_test "order build option"       order build option
run_help_test "order build bracket"      order build bracket
run_help_test "order build oco"          order build oco
run_help_test "order build vertical"     order build vertical
run_help_test "order build iron-condor"  order build iron-condor
run_help_test "order build straddle"     order build straddle
run_help_test "order build strangle"     order build strangle
run_help_test "order build covered-call" order build covered-call
run_help_test "order build collar"       order build collar
run_help_test "order build calendar"     order build calendar
run_help_test "order build diagonal"     order build diagonal
run_help_test "order build butterfly"    order build butterfly
run_help_test "order build condor"       order build condor
run_help_test "order build vertical-roll" order build vertical-roll
run_help_test "order build back-ratio"   order build back-ratio
run_help_test "order build double-diagonal" order build double-diagonal
run_help_test "order build fts"          order build fts
run_help_test "chain"             chain
run_help_test "chain get"         chain get
run_help_test "chain expiration"  chain expiration
run_help_test "history"           history
run_help_test "history get"       history get
run_help_test "instrument"        instrument
run_help_test "instrument search" instrument search
run_help_test "instrument get"    instrument get
run_help_test "market"            market
run_help_test "market hours"      market hours
run_help_test "market movers"     market movers
run_help_test "symbol"            symbol
run_help_test "symbol build"      symbol build
run_help_test "symbol parse"      symbol parse
run_help_test "ta"                ta
run_help_test "ta sma"            ta sma
run_help_test "ta ema"            ta ema
run_help_test "ta rsi"            ta rsi
run_help_test "ta macd"           ta macd
run_help_test "ta atr"            ta atr
run_help_test "ta bbands"         ta bbands
run_help_test "ta stoch"          ta stoch
run_help_test "ta adx"            ta adx
run_help_test "ta vwap"           ta vwap
run_help_test "ta hv"             ta hv
run_help_test "ta expected-move"  ta expected-move
run_help_test "indicators"        indicators
run_help_test "analyze"           analyze
run_help_test "price-history"     price-history
run_help_test "price-history get" price-history get

# -------------------------------------------------------------------
# Shorthand and alias error cases (no args)
# -------------------------------------------------------------------

section "Shorthand/Alias Error Cases"

run_error_test "quote (no args)" quote
run_error_test "history (no args)" history
run_error_test "ta (no args)" ta
run_error_test "indicators (no args)" indicators
run_error_test "analyze (no args)" analyze

# Shorthand dispatch tests: verify dispatch happens (not exit 1 = validation error)
run_dispatch_test "quote shorthand dispatches" quote AAPL
run_dispatch_test "history shorthand dispatches" history AAPL
run_dispatch_test "ta shorthand dispatches" ta AAPL

# -------------------------------------------------------------------
# Symbol build / parse
# -------------------------------------------------------------------

section "Symbol Build/Parse"

run_test "symbol build: AAPL call" \
    symbol build --underlying AAPL --expiration 2026-06-19 --strike 200 --call
run_test "symbol build: AAPL put" \
    symbol build --underlying AAPL --expiration 2026-06-19 --strike 200 --put
run_test "symbol build: SPY put decimal strike" \
    symbol build --underlying SPY --expiration 2026-12-18 --strike 450.50 --put
run_test "symbol build: single-char underlying" \
    symbol build --underlying X --expiration 2026-06-19 --strike 1.50 --call

run_test "symbol parse: AAPL call" \
    symbol parse "AAPL  260619C00200000"
run_test "symbol parse: SPY put decimal" \
    symbol parse "SPY   261218P00450500"
run_test "symbol parse: single-char underlying" \
    symbol parse "X     260619C00001500"

# -------------------------------------------------------------------
# Order build: Equity - every order type and instruction
# -------------------------------------------------------------------

section "Order Build: Equity"

# Order types
run_build_test "equity: BUY MARKET" \
    order build equity --symbol AAPL --action BUY --quantity 10
run_build_test "equity: BUY LIMIT" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200
run_build_test "equity: BUY STOP" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type STOP --stop-price 190
run_build_test "equity: BUY STOP_LIMIT" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type STOP_LIMIT --price 200 --stop-price 195
run_build_test "equity: BUY TRAILING_STOP" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type TRAILING_STOP \
    --stop-offset 5 --stop-link-type VALUE --stop-type STANDARD
run_build_test "equity: BUY MARKET_ON_CLOSE" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type MARKET_ON_CLOSE

# Instructions
run_build_test "equity: SELL MARKET" \
    order build equity --symbol AAPL --action SELL --quantity 10
run_build_test "equity: SELL LIMIT" \
    order build equity --symbol AAPL --action SELL --quantity 10 --type LIMIT --price 200
run_build_test "equity: SELL_SHORT MARKET" \
    order build equity --symbol AAPL --action SELL_SHORT --quantity 10
run_build_test "equity: BUY_TO_COVER MARKET" \
    order build equity --symbol AAPL --action BUY_TO_COVER --quantity 10

# Duration variations
run_build_test "equity: BUY LIMIT DAY" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --duration DAY
run_build_test "equity: BUY LIMIT GTC" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --duration GOOD_TILL_CANCEL
run_build_test "equity: BUY LIMIT FOK" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --duration FILL_OR_KILL
run_build_test "equity: BUY LIMIT IOC" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --duration IMMEDIATE_OR_CANCEL

# Session variations
run_build_test "equity: BUY LIMIT session AM" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --session AM
run_build_test "equity: BUY LIMIT session PM" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --session PM
run_build_test "equity: BUY LIMIT session SEAMLESS" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 --session SEAMLESS

# Special instructions
run_build_test "equity: BUY LIMIT ALL_OR_NONE" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 \
    --special-instruction ALL_OR_NONE
run_build_test "equity: BUY LIMIT DO_NOT_REDUCE" \
    order build equity --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 \
    --special-instruction DO_NOT_REDUCE

# -------------------------------------------------------------------
# Order build: Option - call/put x instruction permutations
# -------------------------------------------------------------------

section "Order Build: Option"

run_build_test "option: CALL BUY_TO_OPEN MARKET" \
    order build option --underlying AAPL --expiration 2026-06-19 --strike 200 --call \
    --action BUY_TO_OPEN --quantity 1
run_build_test "option: PUT BUY_TO_OPEN MARKET" \
    order build option --underlying AAPL --expiration 2026-06-19 --strike 200 --put \
    --action BUY_TO_OPEN --quantity 1
run_build_test "option: CALL SELL_TO_OPEN LIMIT" \
    order build option --underlying AAPL --expiration 2026-06-19 --strike 200 --call \
    --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 5
run_build_test "option: PUT SELL_TO_OPEN LIMIT" \
    order build option --underlying AAPL --expiration 2026-06-19 --strike 200 --put \
    --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 5
run_build_test "option: CALL BUY_TO_CLOSE MARKET" \
    order build option --underlying AAPL --expiration 2026-06-19 --strike 200 --call \
    --action BUY_TO_CLOSE --quantity 1
run_build_test "option: PUT BUY_TO_CLOSE MARKET" \
    order build option --underlying AAPL --expiration 2026-06-19 --strike 200 --put \
    --action BUY_TO_CLOSE --quantity 1
run_build_test "option: CALL SELL_TO_CLOSE LIMIT" \
    order build option --underlying AAPL --expiration 2026-06-19 --strike 200 --call \
    --action SELL_TO_CLOSE --quantity 1 --type LIMIT --price 5
run_build_test "option: PUT SELL_TO_CLOSE LIMIT" \
    order build option --underlying AAPL --expiration 2026-06-19 --strike 200 --put \
    --action SELL_TO_CLOSE --quantity 1 --type LIMIT --price 5

# Duration and session on options
run_build_test "option: CALL BUY_TO_OPEN LIMIT GTC" \
    order build option --underlying AAPL --expiration 2026-06-19 --strike 200 --call \
    --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5 --duration GOOD_TILL_CANCEL
run_build_test "option: PUT SELL_TO_OPEN LIMIT DAY AM" \
    order build option --underlying AAPL --expiration 2026-06-19 --strike 200 --put \
    --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 5 --duration DAY --session AM

# -------------------------------------------------------------------
# Order build: Bracket - action x exit-type permutations
# -------------------------------------------------------------------

section "Order Build: Bracket"

run_build_test "bracket: BUY MARKET + TP + SL" \
    order build bracket --symbol AAPL --action BUY --quantity 10 --take-profit 220 --stop-loss 180
run_build_test "bracket: BUY MARKET + TP only" \
    order build bracket --symbol AAPL --action BUY --quantity 10 --take-profit 220
run_build_test "bracket: BUY MARKET + SL only" \
    order build bracket --symbol AAPL --action BUY --quantity 10 --stop-loss 180
run_build_test "bracket: SELL MARKET + TP + SL" \
    order build bracket --symbol AAPL --action SELL --quantity 10 --take-profit 180 --stop-loss 220
run_build_test "bracket: BUY LIMIT + TP + SL" \
    order build bracket --symbol AAPL --action BUY --quantity 10 --type LIMIT --price 200 \
    --take-profit 220 --stop-loss 180

# -------------------------------------------------------------------
# Order build: OCO - action x exit-type permutations
# -------------------------------------------------------------------

section "Order Build: OCO"

run_build_test "oco: SELL + TP + SL" \
    order build oco --symbol AAPL --action SELL --quantity 10 --take-profit 220 --stop-loss 180
run_build_test "oco: SELL + TP only" \
    order build oco --symbol AAPL --action SELL --quantity 10 --take-profit 220
run_build_test "oco: SELL + SL only" \
    order build oco --symbol AAPL --action SELL --quantity 10 --stop-loss 180
run_build_test "oco: BUY + TP + SL" \
    order build oco --symbol AAPL --action BUY --quantity 10 --take-profit 180 --stop-loss 220

# -------------------------------------------------------------------
# Order build: Vertical - call/put x open/close
# -------------------------------------------------------------------

section "Order Build: Vertical"

run_build_test "vertical: CALL OPEN" \
    order build vertical --underlying F --expiration 2026-06-18 \
    --long-strike 12 --short-strike 14 --call --open --quantity 1 --price 0.50
run_build_test "vertical: CALL CLOSE" \
    order build vertical --underlying F --expiration 2026-06-18 \
    --long-strike 12 --short-strike 14 --call --close --quantity 1 --price 0.50
run_build_test "vertical: PUT OPEN" \
    order build vertical --underlying F --expiration 2026-06-18 \
    --long-strike 14 --short-strike 12 --put --open --quantity 1 --price 0.50
run_build_test "vertical: PUT CLOSE" \
    order build vertical --underlying F --expiration 2026-06-18 \
    --long-strike 14 --short-strike 12 --put --close --quantity 1 --price 0.50

# -------------------------------------------------------------------
# Order build: Iron Condor - open/close
# -------------------------------------------------------------------

section "Order Build: Iron Condor"

run_build_test "iron-condor: OPEN" \
    order build iron-condor --underlying F --expiration 2026-06-18 \
    --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 \
    --open --quantity 1 --price 0.50
run_build_test "iron-condor: CLOSE" \
    order build iron-condor --underlying F --expiration 2026-06-18 \
    --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 \
    --close --quantity 1 --price 0.50

# -------------------------------------------------------------------
# Order build: Straddle - buy/sell x open/close
# -------------------------------------------------------------------

section "Order Build: Straddle"

run_build_test "straddle: BUY OPEN" \
    order build straddle --underlying F --expiration 2026-06-18 --strike 12 \
    --buy --open --quantity 1 --price 1.50
run_build_test "straddle: BUY CLOSE" \
    order build straddle --underlying F --expiration 2026-06-18 --strike 12 \
    --buy --close --quantity 1 --price 1.50
run_build_test "straddle: SELL OPEN" \
    order build straddle --underlying F --expiration 2026-06-18 --strike 12 \
    --sell --open --quantity 1 --price 1.50
run_build_test "straddle: SELL CLOSE" \
    order build straddle --underlying F --expiration 2026-06-18 --strike 12 \
    --sell --close --quantity 1 --price 1.50

# -------------------------------------------------------------------
# Order build: Strangle - buy/sell x open/close
# -------------------------------------------------------------------

section "Order Build: Strangle"

run_build_test "strangle: BUY OPEN" \
    order build strangle --underlying F --expiration 2026-06-18 \
    --call-strike 14 --put-strike 10 --buy --open --quantity 1 --price 0.50
run_build_test "strangle: BUY CLOSE" \
    order build strangle --underlying F --expiration 2026-06-18 \
    --call-strike 14 --put-strike 10 --buy --close --quantity 1 --price 0.50
run_build_test "strangle: SELL OPEN" \
    order build strangle --underlying F --expiration 2026-06-18 \
    --call-strike 14 --put-strike 10 --sell --open --quantity 1 --price 0.50
run_build_test "strangle: SELL CLOSE" \
    order build strangle --underlying F --expiration 2026-06-18 \
    --call-strike 14 --put-strike 10 --sell --close --quantity 1 --price 0.50

# -------------------------------------------------------------------
# Order build: Covered Call (no constraint permutations)
# -------------------------------------------------------------------

section "Order Build: Covered Call"

run_build_test "covered-call: basic" \
    order build covered-call --underlying F --expiration 2026-06-18 \
    --strike 14 --quantity 1 --price 12.00

# -------------------------------------------------------------------
# Order build: Collar - open/close
# -------------------------------------------------------------------

section "Order Build: Collar"

run_build_test "collar: OPEN" \
    order build collar --underlying F --put-strike 10 --call-strike 14 \
    --expiration 2026-06-18 --quantity 1 --open --price 12.00
run_build_test "collar: CLOSE" \
    order build collar --underlying F --put-strike 10 --call-strike 14 \
    --expiration 2026-06-18 --quantity 1 --close --price 12.00

# -------------------------------------------------------------------
# Order build: Calendar - call/put x open/close
# -------------------------------------------------------------------

section "Order Build: Calendar"

run_build_test "calendar: CALL OPEN" \
    order build calendar --underlying F \
    --near-expiration 2026-05-15 --far-expiration 2026-07-17 \
    --strike 12 --call --open --quantity 1 --price 0.50
run_build_test "calendar: CALL CLOSE" \
    order build calendar --underlying F \
    --near-expiration 2026-05-15 --far-expiration 2026-07-17 \
    --strike 12 --call --close --quantity 1 --price 0.50
run_build_test "calendar: PUT OPEN" \
    order build calendar --underlying F \
    --near-expiration 2026-05-15 --far-expiration 2026-07-17 \
    --strike 12 --put --open --quantity 1 --price 0.50
run_build_test "calendar: PUT CLOSE" \
    order build calendar --underlying F \
    --near-expiration 2026-05-15 --far-expiration 2026-07-17 \
    --strike 12 --put --close --quantity 1 --price 0.50

# -------------------------------------------------------------------
# Order build: Diagonal - call/put x open/close
# -------------------------------------------------------------------

section "Order Build: Diagonal"

run_build_test "diagonal: CALL OPEN" \
    order build diagonal --underlying F \
    --near-strike 12 --far-strike 14 \
    --near-expiration 2026-05-15 --far-expiration 2026-07-17 \
    --call --open --quantity 1 --price 0.50
run_build_test "diagonal: CALL CLOSE" \
    order build diagonal --underlying F \
    --near-strike 12 --far-strike 14 \
    --near-expiration 2026-05-15 --far-expiration 2026-07-17 \
    --call --close --quantity 1 --price 0.50
run_build_test "diagonal: PUT OPEN" \
    order build diagonal --underlying F \
    --near-strike 14 --far-strike 12 \
    --near-expiration 2026-05-15 --far-expiration 2026-07-17 \
    --put --open --quantity 1 --price 0.50
run_build_test "diagonal: PUT CLOSE" \
    order build diagonal --underlying F \
    --near-strike 14 --far-strike 12 \
    --near-expiration 2026-05-15 --far-expiration 2026-07-17 \
    --put --close --quantity 1 --price 0.50

# -------------------------------------------------------------------
# Order build: Additional option strategies
# -------------------------------------------------------------------

section "Order Build: Additional Option Strategies"

run_build_test "butterfly: CALL BUY OPEN" \
    order build butterfly --underlying F --expiration 2026-06-18 \
    --lower-strike 10 --middle-strike 12 --upper-strike 14 \
    --call --buy --open --quantity 1 --price 0.50
run_build_test "condor: PUT SELL OPEN" \
    order build condor --underlying F --expiration 2026-06-18 \
    --lower-strike 10 --lower-middle-strike 12 --upper-middle-strike 14 --upper-strike 16 \
    --put --sell --open --quantity 1 --price 0.75
run_build_test "vertical-roll: CALL CREDIT" \
    order build vertical-roll --underlying F \
    --close-expiration 2026-06-18 --open-expiration 2026-07-17 \
    --close-long-strike 12 --close-short-strike 14 --open-long-strike 13 --open-short-strike 15 \
    --call --credit --quantity 1 --price 0.25
run_build_test "back-ratio: CALL DEBIT" \
    order build back-ratio --underlying F --expiration 2026-06-18 \
    --short-strike 12 --long-strike 14 --call --open --quantity 1 --long-ratio 2 --debit --price 0.20
run_build_test "double-diagonal: OPEN" \
    order build double-diagonal --underlying F \
    --near-expiration 2026-06-18 --far-expiration 2026-07-17 \
    --put-far-strike 9 --put-near-strike 10 --call-near-strike 14 --call-far-strike 15 \
    --open --quantity 1 --price 0.80

# -------------------------------------------------------------------
# Order build: FTS (first-triggers-second)
# -------------------------------------------------------------------

section "Order Build: FTS"

# Build a primary entry order and secondary exit order using temp files.
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

"$BINARY" order build equity --symbol AAPL --action BUY --quantity 10 \
    --type LIMIT --price 200 > "$TMPDIR/primary.json" 2>&1
"$BINARY" order build equity --symbol AAPL --action SELL --quantity 10 \
    --type STOP --stop-price 190 > "$TMPDIR/secondary.json" 2>&1

run_build_test "fts: equity entry + stop exit" \
    order build fts --primary "@$TMPDIR/primary.json" --secondary "@$TMPDIR/secondary.json"

fi  # end TIER 1

# ===================================================================
# TIER 2: Auth required (read-only)
# ===================================================================

if [ "$TIER" = "all" ] || [ "$TIER" = "2" ]; then

# Check for valid auth before running tier 2.
if "$BINARY" auth status > /dev/null 2>&1; then

printf "\n${BOLD}=== TIER 2: Auth Required (Read-Only) ===${NC}\n"

# -------------------------------------------------------------------
# Auth
# -------------------------------------------------------------------

section "Auth"

run_test "auth status" auth status

# -------------------------------------------------------------------
# Account
# -------------------------------------------------------------------

section "Account"

run_test "account summary" account summary
run_test "account summary --positions" account summary --positions
run_test "account list" account list
run_test "account numbers" account numbers
run_test "account list --positions" account list --positions

# Extract the first account hash for account-specific commands.
ACCOUNT_HASH=$("$BINARY" account numbers 2>/dev/null | jq -r '.data.accounts[0].hashValue // empty' 2>/dev/null)

if [ -n "$ACCOUNT_HASH" ]; then
    run_test "account get" account get --account "$ACCOUNT_HASH"
    run_test "account get --positions" account get --account "$ACCOUNT_HASH" --positions
    run_test "account transaction list" account transaction list --account "$ACCOUNT_HASH"
else
    printf "${YELLOW}SKIP${NC} account get/transaction (no account hash found)\n"
    SKIP=$((SKIP + 1))
fi

# -------------------------------------------------------------------
# Position
# -------------------------------------------------------------------

section "Position"

run_test "position list" position list
run_test "position list --all-accounts" position list --all-accounts

# -------------------------------------------------------------------
# Quote
# -------------------------------------------------------------------

section "Quote"

run_test "quote get: single" quote get AAPL
run_test "quote get: multi" quote get AAPL NVDA TSLA
run_test "quote get: with fields" quote get AAPL --fields quote,fundamental

# -------------------------------------------------------------------
# Order list (read-only)
# -------------------------------------------------------------------

section "Order (read-only)"

run_test "order list" order list
run_test "order list --status all" order list --status all

# -------------------------------------------------------------------
# Chain
# -------------------------------------------------------------------

section "Chain"

run_test "chain get" chain get AAPL
run_test "chain get: CALL only" chain get AAPL --type CALL
run_test "chain get: PUT only" chain get AAPL --type PUT
run_test "chain expiration" chain expiration AAPL

# -------------------------------------------------------------------
# History
# -------------------------------------------------------------------

section "History"

run_test "history get: defaults" history get AAPL
run_test "history get: monthly" history get AAPL --period-type month --period 3 --frequency-type daily --frequency 1

# -------------------------------------------------------------------
# Instrument
# -------------------------------------------------------------------

section "Instrument"

run_test "instrument search" instrument search AAPL
run_test "instrument search: desc" instrument search Apple --projection desc-search
run_test "instrument get: AAPL CUSIP" instrument get 037833100

# -------------------------------------------------------------------
# Market
# -------------------------------------------------------------------

section "Market"

run_test "market hours: all" market hours
run_test "market hours: equity" market hours equity
run_test "market movers" market movers '$SPX'

# -------------------------------------------------------------------
# Technical Analysis (all 11 indicators)
# -------------------------------------------------------------------

section "Technical Analysis"

run_test "ta sma" ta sma AAPL
run_test "ta ema" ta ema AAPL
run_test "ta rsi" ta rsi AAPL
run_test "ta macd" ta macd AAPL
run_test "ta atr" ta atr AAPL
run_test "ta bbands" ta bbands AAPL
run_test "ta stoch" ta stoch AAPL
run_test "ta adx" ta adx AAPL
run_test "ta vwap" ta vwap AAPL
run_test "ta hv" ta hv AAPL
run_test "ta expected-move" ta expected-move AAPL

else
    printf "\n${YELLOW}Skipping Tier 2: no valid auth token (run 'schwab-agent auth login' first)${NC}\n"
fi  # end auth check

fi  # end TIER 2

# ===================================================================
# Summary
# ===================================================================

TOTAL=$((PASS + FAIL + SKIP))
printf "\n${BOLD}=== Results ===${NC}\n"
printf "Total: %d\n" "$TOTAL"
printf "${GREEN}Pass:  %d${NC}\n" "$PASS"
if [ "$FAIL" -gt 0 ]; then
    printf "${RED}Fail:  %d${NC}\n" "$FAIL"
else
    printf "Fail:  0\n"
fi
if [ "$SKIP" -gt 0 ]; then
    printf "${YELLOW}Skip:  %d${NC}\n" "$SKIP"
fi

# Exit non-zero if any test failed.
[ "$FAIL" -eq 0 ]

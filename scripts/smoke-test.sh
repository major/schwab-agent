#!/bin/bash
# Smoke tests for schwab-agent CLI.
#
# Tier 1 (no auth): help text, symbol build/parse, all order build permutations.
#   Runs in CI and locally.
#
# Tier 2 (auth required): read-only API commands (account, quote, option, etc.).
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

# Dynamic future expiration dates for option order tests. Using computed dates
# ensures tests don't fail as calendar dates pass. Symbol build/parse tests
# keep hardcoded dates since they don't validate expiration.
FUTURE_EXP=$(date -d "+1 year" +%Y-%m-%d)
FUTURE_EXP_FAR=$(date -d "+18 months" +%Y-%m-%d)

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

# run_help_contains_test validates that --help includes specific command surface
# text. This catches regressions where a command exists but required flags or
# subcommands disappear from the visible CLI contract.
run_help_contains_test() {
    local name="$1"
    local expected="$2"
    shift 2
    local output
    if output=$("$BINARY" "$@" --help 2>&1); then
        if printf "%s" "$output" | grep -F -- "$expected" > /dev/null 2>&1; then
            printf "${GREEN}PASS${NC} %s --help contains %s\n" "$name" "$expected"
            PASS=$((PASS + 1))
        else
            printf "${RED}FAIL${NC} %s --help missing %s\n" "$name" "$expected"
            FAIL=$((FAIL + 1))
        fi
    else
        local rc=$?
        printf "${RED}FAIL${NC} %s --help (exit %d)\n" "$name" "$rc"
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
run_help_test "account resolve" account resolve
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
run_help_test "order replace option" order replace option
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
run_help_test "order build long-call"           order build long-call
run_help_test "order build long-put"            order build long-put
run_help_test "order build cash-secured-put"    order build cash-secured-put
run_help_test "order build naked-call"          order build naked-call
run_help_test "order build sell-covered-call"   order build sell-covered-call
run_help_test "order build put-credit-spread"   order build put-credit-spread
run_help_test "order build call-credit-spread"  order build call-credit-spread
run_help_test "order build put-debit-spread"    order build put-debit-spread
run_help_test "order build call-debit-spread"   order build call-debit-spread
run_help_test "order build long-straddle"       order build long-straddle
run_help_test "order build short-straddle"      order build short-straddle
run_help_test "order build long-strangle"       order build long-strangle
run_help_test "order build short-strangle"      order build short-strangle
run_help_test "order build short-iron-condor"   order build short-iron-condor
run_help_test "order build jade-lizard"         order build jade-lizard
run_help_test "order place long-call"           order place long-call
run_help_test "order place long-put"            order place long-put
run_help_test "order place cash-secured-put"    order place cash-secured-put
run_help_test "order place naked-call"          order place naked-call
run_help_test "order place sell-covered-call"   order place sell-covered-call
run_help_test "order place put-credit-spread"   order place put-credit-spread
run_help_test "order place call-credit-spread"  order place call-credit-spread
run_help_test "order place put-debit-spread"    order place put-debit-spread
run_help_test "order place call-debit-spread"   order place call-debit-spread
run_help_test "order place long-straddle"       order place long-straddle
run_help_test "order place short-straddle"      order place short-straddle
run_help_test "order place long-strangle"       order place long-strangle
run_help_test "order place short-strangle"      order place short-strangle
run_help_test "order place short-iron-condor"   order place short-iron-condor
run_help_test "order place jade-lizard"         order place jade-lizard
run_help_test "order preview long-call"         order preview long-call
run_help_test "order preview long-put"          order preview long-put
run_help_test "order preview cash-secured-put"  order preview cash-secured-put
run_help_test "order preview naked-call"        order preview naked-call
run_help_test "order preview sell-covered-call" order preview sell-covered-call
run_help_test "order preview put-credit-spread" order preview put-credit-spread
run_help_test "order preview call-credit-spread" order preview call-credit-spread
run_help_test "order preview put-debit-spread"  order preview put-debit-spread
run_help_test "order preview call-debit-spread" order preview call-debit-spread
run_help_test "order preview long-straddle"     order preview long-straddle
run_help_test "order preview short-straddle"    order preview short-straddle
run_help_test "order preview long-strangle"     order preview long-strangle
run_help_test "order preview short-strangle"    order preview short-strangle
run_help_test "order preview short-iron-condor" order preview short-iron-condor
run_help_test "order preview jade-lizard"       order preview jade-lizard
run_help_test "option"             option
run_help_test "option expirations" option expirations
run_help_test "option chain"       option chain
run_help_test "option contract"    option contract
run_help_test "option screen"      option screen
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

run_help_contains_test "order replace lists option subcommand" "option" order replace
run_help_contains_test "order replace shows equity symbol flag" "--symbol" order replace
run_help_contains_test "order replace option shows underlying flag" "--underlying" order replace option
run_help_contains_test "order replace option shows call flag" "--call" order replace option
run_help_contains_test "order replace option shows put flag" "--put" order replace option
run_help_contains_test "order preview shows save-preview flag" "--save-preview" order preview
run_help_contains_test "order preview equity shows save-preview flag" "--save-preview" order preview equity
run_help_contains_test "order preview sell-covered-call shows save-preview flag" "--save-preview" order preview sell-covered-call
run_help_contains_test "order place shows from-preview flag" "--from-preview" order place

run_help_contains_test "option chain shows --dte flag" "--dte" option chain
run_help_contains_test "option chain shows --delta-min flag" "--delta-min" option chain
run_help_contains_test "option chain shows --delta-max flag" "--delta-max" option chain
run_help_contains_test "option chain shows --fields flag" "--fields" option chain
run_help_contains_test "option chain shows compact delta example" "--fields strike,delta,bid,ask,mid,openInterest,totalVolume,volatility,daysToExpiration" option chain
run_help_contains_test "option chain shows --strike-count flag" "--strike-count" option chain
run_help_contains_test "option contract shows --expiration flag" "--expiration" option contract
run_help_contains_test "option contract shows --call flag" "--call" option contract
run_help_contains_test "option contract shows --put flag" "--put" option contract
run_help_contains_test "option screen shows --delta-min flag" "--delta-min" option screen
run_help_contains_test "option screen shows --min-bid flag" "--min-bid" option screen
run_help_contains_test "option screen shows --max-spread-pct flag" "--max-spread-pct" option screen
run_help_contains_test "option screen shows --sort flag" "--sort" option screen
run_help_contains_test "option screen shows --limit flag" "--limit" option screen

run_help_contains_test "quote get shows underlying flag" "--underlying" quote get
run_help_contains_test "quote get shows call flag" "--call" quote get
run_help_contains_test "quote get shows put flag" "--put" quote get

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
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --call \
    --action BUY_TO_OPEN --quantity 1
run_build_test "option: PUT BUY_TO_OPEN MARKET" \
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --put \
    --action BUY_TO_OPEN --quantity 1
run_build_test "option: CALL SELL_TO_OPEN LIMIT" \
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --call \
    --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 5
run_build_test "option: PUT SELL_TO_OPEN LIMIT" \
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --put \
    --action SELL_TO_OPEN --quantity 1 --type LIMIT --price 5
run_build_test "option: CALL BUY_TO_CLOSE MARKET" \
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --call \
    --action BUY_TO_CLOSE --quantity 1
run_build_test "option: PUT BUY_TO_CLOSE MARKET" \
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --put \
    --action BUY_TO_CLOSE --quantity 1
run_build_test "option: CALL SELL_TO_CLOSE LIMIT" \
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --call \
    --action SELL_TO_CLOSE --quantity 1 --type LIMIT --price 5
run_build_test "option: PUT SELL_TO_CLOSE LIMIT" \
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --put \
    --action SELL_TO_CLOSE --quantity 1 --type LIMIT --price 5

# Duration and session on options
run_build_test "option: CALL BUY_TO_OPEN LIMIT GTC" \
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --call \
    --action BUY_TO_OPEN --quantity 1 --type LIMIT --price 5 --duration GOOD_TILL_CANCEL
run_build_test "option: PUT SELL_TO_OPEN LIMIT DAY AM" \
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --put \
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
# Order build: Buy-With-Stop - entry + protective stop (+ optional TP)
# -------------------------------------------------------------------

section "Order Build: Buy-With-Stop"

# Help tests
run_help_test "order place buy-with-stop"    order place buy-with-stop
run_help_test "order build buy-with-stop"    order build buy-with-stop
run_help_test "order preview buy-with-stop"  order preview buy-with-stop

# Build tests
run_build_test "buy-with-stop: LIMIT + SL" \
    order build buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140
run_build_test "buy-with-stop: MARKET + SL" \
    order build buy-with-stop --symbol AAPL --quantity 10 --type MARKET --stop-loss 140
run_build_test "buy-with-stop: LIMIT + SL + TP" \
    order build buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140 --take-profit 170
run_build_test "buy-with-stop: MARKET + SL + TP" \
    order build buy-with-stop --symbol AAPL --quantity 10 --type MARKET --stop-loss 140 --take-profit 170
run_build_test "buy-with-stop: LIMIT + SL + GTC" \
    order build buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140 --duration GOOD_TILL_CANCEL
run_build_test "buy-with-stop: alias --order-type MARKET" \
    order build buy-with-stop --symbol AAPL --quantity 10 --order-type MARKET --stop-loss 140

# Error tests (expect non-zero exit code)
run_error_test "buy-with-stop: missing --stop-loss" \
    order build buy-with-stop --symbol AAPL --quantity 10 --price 150
run_error_test "buy-with-stop: invalid --type STOP" \
    order build buy-with-stop --symbol AAPL --quantity 10 --price 150 --stop-loss 140 --type STOP

# -------------------------------------------------------------------
# Order build: Vertical - call/put x open/close
# -------------------------------------------------------------------

section "Order Build: Vertical"

run_build_test "vertical: CALL OPEN" \
    order build vertical --underlying F --expiration $FUTURE_EXP \
    --long-strike 12 --short-strike 14 --call --open --quantity 1 --price 0.50
run_build_test "vertical: CALL CLOSE" \
    order build vertical --underlying F --expiration $FUTURE_EXP \
    --long-strike 12 --short-strike 14 --call --close --quantity 1 --price 0.50
run_build_test "vertical: PUT OPEN" \
    order build vertical --underlying F --expiration $FUTURE_EXP \
    --long-strike 14 --short-strike 12 --put --open --quantity 1 --price 0.50
run_build_test "vertical: PUT CLOSE" \
    order build vertical --underlying F --expiration $FUTURE_EXP \
    --long-strike 14 --short-strike 12 --put --close --quantity 1 --price 0.50

# -------------------------------------------------------------------
# Order build: Iron Condor - open/close
# -------------------------------------------------------------------

section "Order Build: Iron Condor"

run_build_test "iron-condor: OPEN" \
    order build iron-condor --underlying F --expiration $FUTURE_EXP \
    --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 \
    --open --quantity 1 --price 0.50
run_build_test "iron-condor: CLOSE" \
    order build iron-condor --underlying F --expiration $FUTURE_EXP \
    --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 \
    --close --quantity 1 --price 0.50

# -------------------------------------------------------------------
# Order build: Straddle - buy/sell x open/close
# -------------------------------------------------------------------

section "Order Build: Straddle"

run_build_test "straddle: BUY OPEN" \
    order build straddle --underlying F --expiration $FUTURE_EXP --strike 12 \
    --buy --open --quantity 1 --price 1.50
run_build_test "straddle: BUY CLOSE" \
    order build straddle --underlying F --expiration $FUTURE_EXP --strike 12 \
    --buy --close --quantity 1 --price 1.50
run_build_test "straddle: SELL OPEN" \
    order build straddle --underlying F --expiration $FUTURE_EXP --strike 12 \
    --sell --open --quantity 1 --price 1.50
run_build_test "straddle: SELL CLOSE" \
    order build straddle --underlying F --expiration $FUTURE_EXP --strike 12 \
    --sell --close --quantity 1 --price 1.50

# -------------------------------------------------------------------
# Order build: Strangle - buy/sell x open/close
# -------------------------------------------------------------------

section "Order Build: Strangle"

run_build_test "strangle: BUY OPEN" \
    order build strangle --underlying F --expiration $FUTURE_EXP \
    --call-strike 14 --put-strike 10 --buy --open --quantity 1 --price 0.50
run_build_test "strangle: BUY CLOSE" \
    order build strangle --underlying F --expiration $FUTURE_EXP \
    --call-strike 14 --put-strike 10 --buy --close --quantity 1 --price 0.50
run_build_test "strangle: SELL OPEN" \
    order build strangle --underlying F --expiration $FUTURE_EXP \
    --call-strike 14 --put-strike 10 --sell --open --quantity 1 --price 0.50
run_build_test "strangle: SELL CLOSE" \
    order build strangle --underlying F --expiration $FUTURE_EXP \
    --call-strike 14 --put-strike 10 --sell --close --quantity 1 --price 0.50

# -------------------------------------------------------------------
# Order build: Covered Call (no constraint permutations)
# -------------------------------------------------------------------

section "Order Build: Covered Call"

run_build_test "covered-call: basic" \
    order build covered-call --underlying F --expiration $FUTURE_EXP \
    --strike 14 --quantity 1 --price 12.00

# -------------------------------------------------------------------
# Order build: Collar - open/close
# -------------------------------------------------------------------

section "Order Build: Collar"

run_build_test "collar: OPEN" \
    order build collar --underlying F --put-strike 10 --call-strike 14 \
    --expiration $FUTURE_EXP --quantity 1 --open --price 12.00
run_build_test "collar: CLOSE" \
    order build collar --underlying F --put-strike 10 --call-strike 14 \
    --expiration $FUTURE_EXP --quantity 1 --close --price 12.00

# -------------------------------------------------------------------
# Order build: Calendar - call/put x open/close
# -------------------------------------------------------------------

section "Order Build: Calendar"

run_build_test "calendar: CALL OPEN" \
    order build calendar --underlying F \
    --near-expiration $FUTURE_EXP --far-expiration $FUTURE_EXP_FAR \
    --strike 12 --call --open --quantity 1 --price 0.50
run_build_test "calendar: CALL CLOSE" \
    order build calendar --underlying F \
    --near-expiration $FUTURE_EXP --far-expiration $FUTURE_EXP_FAR \
    --strike 12 --call --close --quantity 1 --price 0.50
run_build_test "calendar: PUT OPEN" \
    order build calendar --underlying F \
    --near-expiration $FUTURE_EXP --far-expiration $FUTURE_EXP_FAR \
    --strike 12 --put --open --quantity 1 --price 0.50
run_build_test "calendar: PUT CLOSE" \
    order build calendar --underlying F \
    --near-expiration $FUTURE_EXP --far-expiration $FUTURE_EXP_FAR \
    --strike 12 --put --close --quantity 1 --price 0.50

# -------------------------------------------------------------------
# Order build: Diagonal - call/put x open/close
# -------------------------------------------------------------------

section "Order Build: Diagonal"

run_build_test "diagonal: CALL OPEN" \
    order build diagonal --underlying F \
    --near-strike 12 --far-strike 14 \
    --near-expiration $FUTURE_EXP --far-expiration $FUTURE_EXP_FAR \
    --call --open --quantity 1 --price 0.50
run_build_test "diagonal: CALL CLOSE" \
    order build diagonal --underlying F \
    --near-strike 12 --far-strike 14 \
    --near-expiration $FUTURE_EXP --far-expiration $FUTURE_EXP_FAR \
    --call --close --quantity 1 --price 0.50
run_build_test "diagonal: PUT OPEN" \
    order build diagonal --underlying F \
    --near-strike 14 --far-strike 12 \
    --near-expiration $FUTURE_EXP --far-expiration $FUTURE_EXP_FAR \
    --put --open --quantity 1 --price 0.50
run_build_test "diagonal: PUT CLOSE" \
    order build diagonal --underlying F \
    --near-strike 14 --far-strike 12 \
    --near-expiration $FUTURE_EXP --far-expiration $FUTURE_EXP_FAR \
    --put --close --quantity 1 --price 0.50

# -------------------------------------------------------------------
# Order build: Additional option strategies
# -------------------------------------------------------------------

section "Order Build: Additional Option Strategies"

run_build_test "butterfly: CALL BUY OPEN" \
    order build butterfly --underlying F --expiration $FUTURE_EXP \
    --lower-strike 10 --middle-strike 12 --upper-strike 14 \
    --call --buy --open --quantity 1 --price 0.50
run_build_test "condor: PUT SELL OPEN" \
    order build condor --underlying F --expiration $FUTURE_EXP \
    --lower-strike 10 --lower-middle-strike 12 --upper-middle-strike 14 --upper-strike 16 \
    --put --sell --open --quantity 1 --price 0.75
run_build_test "vertical-roll: CALL CREDIT" \
    order build vertical-roll --underlying F \
    --close-expiration $FUTURE_EXP --open-expiration $FUTURE_EXP_FAR \
    --close-long-strike 12 --close-short-strike 14 --open-long-strike 13 --open-short-strike 15 \
    --call --credit --quantity 1 --price 0.25
run_build_test "back-ratio: CALL DEBIT" \
    order build back-ratio --underlying F --expiration $FUTURE_EXP \
    --short-strike 12 --long-strike 14 --call --open --quantity 1 --long-ratio 2 --debit --price 0.20
run_build_test "double-diagonal: OPEN" \
    order build double-diagonal --underlying F \
    --near-expiration $FUTURE_EXP --far-expiration $FUTURE_EXP_FAR \
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

# -------------------------------------------------------------------
# Order flag aliases: --instruction (alias for --action),
#                     --order-type (alias for --type)
# -------------------------------------------------------------------

section "Order Flag Aliases"

# Help text contains alias flags
run_help_contains_test "build equity --help shows --instruction" "--instruction" order build equity
run_help_contains_test "build equity --help shows --order-type" "--order-type" order build equity
run_help_contains_test "build option --help shows --instruction" "--instruction" order build option
run_help_contains_test "build option --help shows --order-type" "--order-type" order build option
run_help_contains_test "build bracket --help shows --instruction" "--instruction" order build bracket
run_help_contains_test "build bracket --help shows --order-type" "--order-type" order build bracket
run_help_contains_test "build oco --help shows --instruction" "--instruction" order build oco

# Alias flags produce valid order JSON
run_build_test "alias: equity --instruction BUY --order-type MARKET" \
    order build equity --symbol AAPL --instruction BUY --quantity 10 --order-type MARKET
run_build_test "alias: equity --instruction SELL --order-type LIMIT" \
    order build equity --symbol AAPL --instruction SELL --quantity 10 --order-type LIMIT --price 200
run_build_test "alias: option --instruction BUY_TO_OPEN --order-type LIMIT" \
    order build option --underlying AAPL --expiration $FUTURE_EXP --strike 200 --call \
    --instruction BUY_TO_OPEN --quantity 1 --order-type LIMIT --price 5.00
run_build_test "alias: bracket --instruction BUY --order-type MARKET" \
    order build bracket --symbol NVDA --instruction BUY --quantity 10 \
    --order-type MARKET --take-profit 150 --stop-loss 120
run_build_test "alias: oco --instruction SELL (no --type)" \
    order build oco --symbol AAPL --instruction SELL --quantity 100 \
    --take-profit 160 --stop-loss 140

# Conflict: primary and alias set simultaneously
run_error_test "alias conflict: --action + --instruction" \
    order build equity --symbol AAPL --action BUY --instruction SELL --quantity 10
run_error_test "alias conflict: --type + --order-type" \
    order build equity --symbol AAPL --action BUY --quantity 10 \
    --type LIMIT --order-type MARKET --price 200

# Multi-leg strategies must NOT have alias flags
run_error_test "vertical rejects --instruction" \
    order build vertical --underlying F --expiration $FUTURE_EXP \
    --long-strike 12 --short-strike 14 --call --open --quantity 1 --price 0.50 \
    --instruction BUY
run_error_test "iron-condor rejects --order-type" \
    order build iron-condor --underlying F --expiration $FUTURE_EXP \
    --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 \
    --open --quantity 1 --price 0.50 --order-type LIMIT

# -------------------------------------------------------------------
# Order build: Porcelain Single-Leg
# -------------------------------------------------------------------

section "Order Build: Porcelain Single-Leg"

run_build_test "long-call: basic" \
    order build long-call --underlying AAPL --expiration $FUTURE_EXP \
    --strike 200 --quantity 1 --type LIMIT --price 5.00
run_build_test "long-call: MARKET" \
    order build long-call --underlying AAPL --expiration $FUTURE_EXP \
    --strike 200 --quantity 1
run_build_test "long-put: basic" \
    order build long-put --underlying AAPL --expiration $FUTURE_EXP \
    --strike 200 --quantity 1 --type LIMIT --price 3.00
run_build_test "cash-secured-put: basic" \
    order build cash-secured-put --underlying AAPL --expiration $FUTURE_EXP \
    --strike 180 --quantity 1 --type LIMIT --price 4.00
run_build_test "naked-call: basic" \
    order build naked-call --underlying AAPL --expiration $FUTURE_EXP \
    --strike 220 --quantity 1 --type LIMIT --price 2.00
run_build_test "sell-covered-call: basic" \
    order build sell-covered-call --underlying F --expiration $FUTURE_EXP \
    --strike 14 --quantity 1 --type LIMIT --price 1.00
run_build_test "long-call: GTC duration" \
    order build long-call --underlying AAPL --expiration $FUTURE_EXP \
    --strike 200 --quantity 1 --type LIMIT --price 5.00 --duration GOOD_TILL_CANCEL

# Porcelain single-leg must NOT accept direction flags
run_error_test "long-call rejects --action" \
    order build long-call --underlying AAPL --expiration $FUTURE_EXP \
    --strike 200 --quantity 1 --action BUY_TO_OPEN
run_error_test "long-call rejects --call" \
    order build long-call --underlying AAPL --expiration $FUTURE_EXP \
    --strike 200 --quantity 1 --call
run_error_test "sell-covered-call rejects --action" \
    order build sell-covered-call --underlying F --expiration $FUTURE_EXP \
    --strike 14 --quantity 1 --type LIMIT --price 1.00 --action SELL_TO_OPEN

# Help-contains: porcelain shows expected flags
run_help_contains_test "long-call shows --underlying" "--underlying" order build long-call
run_help_contains_test "long-call shows --strike" "--strike" order build long-call
run_help_contains_test "long-call shows --quantity" "--quantity" order build long-call
run_help_contains_test "sell-covered-call shows --strike" "--strike" order build sell-covered-call

# -------------------------------------------------------------------
# Order build: Porcelain Vertical Spread
# -------------------------------------------------------------------

section "Order Build: Porcelain Vertical Spread"

run_build_test "put-credit-spread: basic" \
    order build put-credit-spread --underlying F --expiration $FUTURE_EXP \
    --high-strike 14 --low-strike 12 --quantity 1 --price 0.50
run_build_test "call-credit-spread: basic" \
    order build call-credit-spread --underlying F --expiration $FUTURE_EXP \
    --high-strike 14 --low-strike 12 --quantity 1 --price 0.50
run_build_test "put-debit-spread: basic" \
    order build put-debit-spread --underlying F --expiration $FUTURE_EXP \
    --high-strike 14 --low-strike 12 --quantity 1 --price 0.50
run_build_test "call-debit-spread: basic" \
    order build call-debit-spread --underlying F --expiration $FUTURE_EXP \
    --high-strike 14 --low-strike 12 --quantity 1 --price 0.50
run_build_test "put-credit-spread: GTC" \
    order build put-credit-spread --underlying F --expiration $FUTURE_EXP \
    --high-strike 14 --low-strike 12 --quantity 1 --price 0.50 --duration GOOD_TILL_CANCEL

# Porcelain vertical must NOT accept direction flags
run_error_test "put-credit-spread rejects --call" \
    order build put-credit-spread --underlying F --expiration $FUTURE_EXP \
    --high-strike 14 --low-strike 12 --quantity 1 --price 0.50 --call
run_error_test "put-credit-spread rejects --open" \
    order build put-credit-spread --underlying F --expiration $FUTURE_EXP \
    --high-strike 14 --low-strike 12 --quantity 1 --price 0.50 --open
run_error_test "put-credit-spread rejects --long-strike" \
    order build put-credit-spread --underlying F --expiration $FUTURE_EXP \
    --high-strike 14 --low-strike 12 --quantity 1 --price 0.50 --long-strike 12
run_error_test "put-credit-spread rejects high <= low strike" \
    order build put-credit-spread --underlying F --expiration $FUTURE_EXP \
    --high-strike 12 --low-strike 14 --quantity 1 --price 0.50

# Help-contains: porcelain vertical shows factual strike names
run_help_contains_test "put-credit-spread shows --high-strike" "--high-strike" order build put-credit-spread
run_help_contains_test "put-credit-spread shows --low-strike" "--low-strike" order build put-credit-spread

# -------------------------------------------------------------------
# Order build: Porcelain Straddle/Strangle
# -------------------------------------------------------------------

section "Order Build: Porcelain Straddle/Strangle"

run_build_test "long-straddle: basic" \
    order build long-straddle --underlying F --expiration $FUTURE_EXP \
    --strike 12 --quantity 1 --price 1.50
run_build_test "short-straddle: basic" \
    order build short-straddle --underlying F --expiration $FUTURE_EXP \
    --strike 12 --quantity 1 --price 1.50
run_build_test "long-strangle: basic" \
    order build long-strangle --underlying F --expiration $FUTURE_EXP \
    --call-strike 14 --put-strike 10 --quantity 1 --price 0.50
run_build_test "short-strangle: basic" \
    order build short-strangle --underlying F --expiration $FUTURE_EXP \
    --call-strike 14 --put-strike 10 --quantity 1 --price 0.50

# Porcelain symmetric must NOT accept direction flags
run_error_test "long-straddle rejects --buy" \
    order build long-straddle --underlying F --expiration $FUTURE_EXP \
    --strike 12 --quantity 1 --price 1.50 --buy
run_error_test "long-straddle rejects --open" \
    order build long-straddle --underlying F --expiration $FUTURE_EXP \
    --strike 12 --quantity 1 --price 1.50 --open

# -------------------------------------------------------------------
# Order build: Porcelain Short Iron Condor
# -------------------------------------------------------------------

section "Order Build: Porcelain Short Iron Condor"

run_build_test "short-iron-condor: basic" \
    order build short-iron-condor --underlying F --expiration $FUTURE_EXP \
    --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 \
    --quantity 1 --price 0.50
run_build_test "short-iron-condor: GTC" \
    order build short-iron-condor --underlying F --expiration $FUTURE_EXP \
    --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 \
    --quantity 1 --price 0.50 --duration GOOD_TILL_CANCEL

# Porcelain short iron condor hardcodes opening direction.
run_error_test "short-iron-condor rejects --open" \
    order build short-iron-condor --underlying F --expiration $FUTURE_EXP \
    --put-long-strike 9 --put-short-strike 10 --call-short-strike 14 --call-long-strike 15 \
    --quantity 1 --price 0.50 --open

# Help-contains: short iron condor shows factual strike names.
run_help_contains_test "short-iron-condor shows --put-long-strike" "--put-long-strike" order build short-iron-condor
run_help_contains_test "short-iron-condor shows --call-long-strike" "--call-long-strike" order build short-iron-condor

# -------------------------------------------------------------------
# Order build: Porcelain Jade Lizard
# -------------------------------------------------------------------

section "Order Build: Porcelain Jade Lizard"

run_build_test "jade-lizard: basic" \
    order build jade-lizard --underlying F --expiration $FUTURE_EXP \
    --put-strike 10 --short-call-strike 14 --long-call-strike 16 \
    --quantity 1 --price 1.00
run_build_test "jade-lizard: GTC" \
    order build jade-lizard --underlying F --expiration $FUTURE_EXP \
    --put-strike 10 --short-call-strike 14 --long-call-strike 16 \
    --quantity 1 --price 1.00 --duration GOOD_TILL_CANCEL

# Strike ordering validation
run_error_test "jade-lizard: put >= short-call" \
    order build jade-lizard --underlying F --expiration $FUTURE_EXP \
    --put-strike 14 --short-call-strike 14 --long-call-strike 16 \
    --quantity 1 --price 1.00
run_error_test "jade-lizard: short-call >= long-call" \
    order build jade-lizard --underlying F --expiration $FUTURE_EXP \
    --put-strike 10 --short-call-strike 16 --long-call-strike 14 \
    --quantity 1 --price 1.00

# Help-contains: jade lizard shows its 3-strike flags
run_help_contains_test "jade-lizard shows --put-strike" "--put-strike" order build jade-lizard
run_help_contains_test "jade-lizard shows --short-call-strike" "--short-call-strike" order build jade-lizard
run_help_contains_test "jade-lizard shows --long-call-strike" "--long-call-strike" order build jade-lizard

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

# Extract the first account hash for account-specific commands.
ACCOUNT_HASH=$("$BINARY" account numbers 2>/dev/null | jq -r '.data.accounts[0].hashValue // empty' 2>/dev/null)

if [ -n "$ACCOUNT_HASH" ]; then
    run_test "account resolve" account resolve --account "$ACCOUNT_HASH"
    run_test "account get" account get --account "$ACCOUNT_HASH"
    run_test "account get --positions" account get --account "$ACCOUNT_HASH" --positions
    run_test "account transaction list" account transaction list --account "$ACCOUNT_HASH"
else
    printf "${YELLOW}SKIP${NC} account resolve/get/transaction (no account hash found)\n"
    SKIP=$((SKIP + 1))
fi
run_test "account list --positions" account list --positions

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
# Option (porcelain)
# -------------------------------------------------------------------

section "Option"

run_test "option expirations" option expirations AAPL
run_test "option chain: CALL" option chain AAPL --type CALL --strike-count 5

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

# AGENTS.md - internal/commands

> Leave generous comments when fixing bugs or working around API quirks. Anything that might save a future developer from re-discovering the same issue is worth writing down.

CLI command handlers for schwab-agent. Each file defines one command group (e.g., `quote.go` defines the `quote` command tree). This is the largest package.

## Command Pattern

Every command group exports one public constructor that returns `*cobra.Command`:

```go
func NewQuoteCmd(c *client.Ref, w io.Writer) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "quote",
        Short: "Stock quote operations",
    }
    cmd.AddCommand(newQuoteGetCmd(c, w))
    return cmd
}
```

- Public function returns the parent command (registered in `main.go`'s `buildAppWithDeps()`)
- Private functions return subcommands
- All commands accept `*client.Ref` and `io.Writer` (some also take `configPath string`), except `symbol` which only takes `io.Writer` (no API client needed)
- RunE functions use `*cobra.Command` and `[]string` args from spf13/cobra

### Cobra Tag Flag Pattern

Most command flags are defined with project-owned struct tags and registered by
`defineCobraFlags(cmd, opts)`:

```go
type quoteGetOpts struct {
    Fields     []string `flag:"fields" flagdescr:"Quote fields to return (repeatable): quote, fundamental, extended, reference, regular"`
    Indicative bool     `flag:"indicative" flagdescr:"Request indicative (non-tradeable) quotes"`
    Underlying string   `flag:"underlying" flagdescr:"Underlying symbol for option quote"`
    Expiration string   `flag:"expiration" flagdescr:"Expiration date (YYYY-MM-DD) for option quote"`
    Strike     float64  `flag:"strike" flagdescr:"Strike price for option quote"`
    Call       bool     `flag:"call" flagdescr:"Call option"`
    Put        bool     `flag:"put" flagdescr:"Put option"`
}
```

- `defineCobraFlags(cmd, opts)` binds tagged fields directly to Cobra/pflag values
- `validateCobraOptions(cmd.Context(), opts)` runs optional `Validate(context.Context) []error` hooks after parsing and before handler logic reads option fields
- `defineAndConstrain[O](cmd, opts, exclusivePairs...)` from helpers.go wraps `defineCobraFlags()` + `MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired` for order commands with mutual exclusion constraints
- Root persistent flags (--account, --verbose, --config, --token) remain as manual Cobra registrations

### Flag Aliases

Order subcommands that use `--action` and `--type` also accept `--instruction` and `--order-type` as aliases. `registerOrderFlagAliases()` adds the alias flags with mutual exclusivity constraints. `resolveOrderFlagAliasesViaFlags()` runs before handlers read their bound option structs, copying alias values to canonical flags via `cmd.Flags().Set()`. `RegisterOrderFlagAliasesOnTree(root)` walks the full command tree post-setup to register aliases on all 14 qualifying subcommands. Alias flags use a lowercase "alias for --" Usage prefix so structcli's JSON Schema generator skips them.

## Adding a New Command

1. Create `<name>.go` with a public `<Name>Command()` constructor
2. Register in `cmd/schwab-agent/main.go` inside `buildApp()`
3. Create `<name>_test.go` with tests
4. Add meaningful Long (2-5 sentences) and Example (3-5 examples) fields for agent discoverability

## Output Rules

All command output goes through `internal/output` envelopes:

- `output.WriteSuccess(w, data, output.NewMetadata())` for success
- Return typed errors from `internal/apperr` for failures (the Before hook handles formatting)
- `output.WritePartial(w, data, missing, meta)` when some results succeed and some fail (e.g., multi-symbol quote)

Exception: `order build` commands write raw JSON (not envelope-wrapped).

## Safety Guards

Mutable commands (order place/cancel/replace) enforce a safety check:

```go
if err := requireMutableEnabled(configPath); err != nil { return err }
```

- `requireMutableEnabled`: Checks `i-also-like-to-live-dangerously` in config
- `preview_ledger.go`: Saves `order preview --save-preview` payloads in the local state dir with 15-minute TTLs. `order place --from-preview <digest>` must submit the saved `models.OrderRequest` unchanged after digest, TTL, and optional `--account` checks pass.

## Account Resolution

All commands needing an account use `resolveAccount(c, accountFlag, configPath, positionalArgs)` from account.go:

1. Check `--account` flag first
2. Check positional args (if non-nil, used by `account get`)
3. Fall back to `default_account` from config
4. If the chosen identifier looks like a long hex Schwab hash, return it without API calls
5. Try numeric resolution: call `AccountNumbers()` to match by account number
6. Try nickname resolution: call `UserPreference()` to match by account nickname (case-insensitive)
7. Return `AccountNotFoundError` if none found or multiple nickname matches exist

Pass `nil` for `positionalArgs` when the command doesn't accept positional account arguments.

## Order Workflows

Four order types share a common pattern: parse flags -> validate -> build -> place/preview.

- **Equity**: `parseEquityParams` -> `orderbuilder.ValidateEquityOrder` -> `orderbuilder.BuildEquityOrder`
- **Option**: `parseOptionParams` -> `orderbuilder.ValidateOptionOrder` -> `orderbuilder.BuildOptionOrder`
- **Bracket**: `parseBracketParams` -> `orderbuilder.ValidateBracketOrder` -> `orderbuilder.BuildBracketOrder`
- **OCO**: `parseOCOParams` -> `orderbuilder.ValidateOCOOrder` -> `orderbuilder.BuildOCOOrder`

Spec mode (`--spec`): Accepts inline JSON, `@file` path, or `-` for stdin via `readSpecSource()`.
Preview digest mode (`order preview --save-preview` then `order place --from-preview <digest>`): Binds the canonical order JSON to the account, operation, and endpoint so agents can separate review from mutation without rebuilding the payload.

## Enum Parsing

CLI string inputs are validated against `models` constants via switch statements:

- `parseInstruction()`: BUY, SELL, BUY_TO_COVER, etc.
- `parseOrderType()`: MARKET, LIMIT, STOP, STOP_LIMIT, etc.
- `parseDuration()`: DAY, GOOD_TILL_CANCEL, FILL_OR_KILL, etc.
- `parseSession()`: NORMAL, AM, PM, SEAMLESS
- `parsePutCall()`: Mutually exclusive `--call`/`--put` flags

All return `ValidationError` on invalid input.

## Testing

Test helpers in `helpers_test.go`:

- `runTestCommand(t, cmd, args...)`: Suppresses `os.Exit` and runs the command
- `testClient(t, server)`: Creates `*client.Ref` backed by httptest server
- `jsonServer(body)`: Returns httptest server that always responds with given JSON

Pattern: Create httptest server with expected response -> build command -> run via `runTestCommand` -> decode output -> assert envelope contents.

Build tags: `//go:build task16` (auth), `//go:build task17` (account), etc.

## File Map

| File | Command | Subcommands |
|---|---|---|
| auth.go | auth | login, status, refresh |
| account.go | account | summary, list, get, numbers, set-default, transaction (list, get) |
| position.go | position | list (--all-accounts, --account) |
| quote.go | quote | get (positional symbols or structured option flags: --underlying, --expiration, --strike, --call/--put) |
| order.go | order | list (--recent, --status), get |
| order_place.go | order | place (equity/option/bracket/oco/buy-with-stop), preview, cancel, replace (equity parent + option sub) |
| order_buy_with_stop.go | order | place/build/preview buy-with-stop (BUY-only bracket with stop-loss, optional take-profit) |
| order_helpers.go | (shared) | opts structs, enum parsing, flag definitions, validation helpers |
| flag_aliases.go | (shared) | registerOrderFlagAliases, resolveOrderFlagAliasesViaFlags, RegisterOrderFlagAliasesOnTree |
| symbol_builder.go | (shared) | buildOCCSymbol for option commands needing OCC symbol construction |
| order_build.go | order build | equity, option, bracket, oco, vertical, iron-condor, straddle, strangle, covered-call, collar, calendar, diagonal, butterfly, condor, vertical-roll, back-ratio, double-diagonal, fts |
| chain.go | chain | get, expiration |
| option_ticket.go | option | ticket get |
| history.go | history | get (alias: price-history) |
| instrument.go | instrument | search, get |
| market.go | market | hours, movers |
| symbol.go | symbol | build, parse |
| ta.go | ta | sma, ema, rsi, macd, atr, bbands, stoch, adx, vwap, hv, expected-move |
| indicators.go | indicators | (shortcut for ta dashboard) |
| analyze.go | analyze | (quote + ta dashboard composite) |

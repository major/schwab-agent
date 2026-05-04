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

### Struct Tag Flag Pattern

Command flags are defined using structcli struct tags:

```go
type quoteGetOpts struct {
    Symbols []string `flag:"symbol" flagdescr:"Symbol(s) to quote" flagshort:"s"`
    Fields  bool     `flag:"fields" flagdescr:"Include all quote fields"`
}

func (o *quoteGetOpts) Attach(_ *cobra.Command) error { return nil }
```

- `structcli.Define(cmd, opts)` in command setup replaces manual `cmd.Flags()` calls
- `structcli.Unmarshal(cmd, opts)` at top of RunE before reading option fields
- `defineAndConstrain[O](cmd, opts, exclusivePairs...)` from helpers.go wraps `Define()` + `MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired` for order commands with mutual exclusion constraints
- Root persistent flags (--account, --verbose, --config, --token) remain as manual Cobra registrations

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

## Account Resolution

All commands needing an account use `resolveAccount(c, accountFlag, configPath, positionalArgs)` from account.go:

1. Check `--account` flag first
2. Check positional args (if non-nil, used by `account get`)
3. Fall back to `default_account` from config
4. If the chosen identifier looks like a long hex Schwab hash, return it without API calls
5. Resolve account numbers through `AccountNumbers()` and nicknames through `UserPreference()`
6. Return `AccountNotFoundError` if none found or multiple nickname matches exist

Pass `nil` for `positionalArgs` when the command doesn't accept positional account arguments.

## Order Workflows

Four order types share a common pattern: parse flags -> validate -> build -> place/preview.

- **Equity**: `parseEquityParams` -> `orderbuilder.ValidateEquityOrder` -> `orderbuilder.BuildEquityOrder`
- **Option**: `parseOptionParams` -> `orderbuilder.ValidateOptionOrder` -> `orderbuilder.BuildOptionOrder`
- **Bracket**: `parseBracketParams` -> `orderbuilder.ValidateBracketOrder` -> `orderbuilder.BuildBracketOrder`
- **OCO**: `parseOCOParams` -> `orderbuilder.ValidateOCOOrder` -> `orderbuilder.BuildOCOOrder`

Spec mode (`--spec`): Accepts inline JSON, `@file` path, or `-` for stdin via `readSpecSource()`.

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
| quote.go | quote | get |
| order.go | order | list (--recent, --status), get |
| order_place.go | order | place (equity/option/bracket/oco), preview, cancel, replace |
| order_helpers.go | (shared) | opts structs, enum parsing, flag definitions, validation helpers |
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

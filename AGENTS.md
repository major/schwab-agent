# AGENTS.md - schwab-agent

> Keep this file and all subdirectory AGENTS.md files updated when the project changes.
> Keep README.md updated whenever the project changes.
> Keep `.coderabbit.yaml` and `.github/copilot-instructions.md` plus `.github/instructions/*.instructions.md` aligned with current repo conventions when review-relevant behavior changes.
> Check /usr/local for newer Go versions before assuming the system Go is current.
> Leave generous comments when fixing bugs or working around API quirks. Anything that might save a future developer from re-discovering the same issue is worth writing down.


## Project

Go CLI tool for AI agents to trade via Charles Schwab APIs. Single binary, JSON-first output, workflow knowledge embedded in command help text.

- **Module**: `github.com/major/schwab-agent`
- **Go version**: 1.26 (check `/usr/local/go/bin/go version` for newer installs)
- **Entry point**: `cmd/schwab-agent/main.go`
- **Dependencies**: spf13/cobra v1.10.2 (CLI framework), pkg/browser (OAuth flow), schwab-go (API and auth helpers), resty.dev/v3 (internal/client HTTP client), stretchr/testify (test assertions)

## Architecture

```text
cmd/schwab-agent/       Entry point, buildApp(), PersistentPreRunE auth hook
internal/
  auth/                 App config plus thin adapters around schwab-go auth helpers
  client/               Schwab API HTTP client (see internal/client/AGENTS.md)
  commands/             CLI command handlers (see internal/commands/AGENTS.md)
  apperr/               Typed error hierarchy with exit codes
  models/               Data structures/schemas for API payloads
  orderbuilder/         Order construction/validation (equity, option, bracket, OCO) + OCC symbol build/parse
  output/               JSON envelope writers (success, error, partial)
```

## Build and Test

```bash
make build       # /usr/local/go/bin/go build -o schwab-agent ./cmd/schwab-agent/
make test        # go test -v ./...
make lint        # golangci-lint run ./...
make smoke       # Both tiers: no-auth + auth-required read-only (local only)
make smoke-ci    # Tier 1 only: no-auth commands (safe for CI)
make install     # Install to /usr/local/bin
make clean       # Remove binary
make release VERSION=vX.Y.Z  # Run test+lint, generate tag message, create GPG-signed tag
```

CI runs lint (golangci-lint v2.11), test (race detector + coverage + build verification), and smoke tests (tier 1) on push to main and PRs. Releases via goreleaser on v* tags (Linux/Darwin, amd64/arm64, CGO disabled).

## Smoke Tests

Shell script at `scripts/smoke-test.sh`. Two tiers:

- **Tier 1** (no auth, CI-safe): help text for all 81 commands/subcommands, symbol build/parse, all 19 order build sub-types with full permutations, flag alias tests, shorthand/alias error cases (205 tests). Runs in CI and locally via `make smoke-ci`.
- **Tier 2** (auth required, local only): read-only API commands (account, position, quote, order list, option expirations, option chain, history, instrument, market, all 11 TA indicators). Requires a valid token. Runs locally via `make smoke` (both tiers) or `SMOKE_TIER=2 ./scripts/smoke-test.sh` (tier 2 only).

Each test validates exit code 0, valid JSON output, and the correct envelope structure (`.data` for API commands, raw JSON for order builds). `SMOKE_BIN` env var overrides the binary path.

## Linting

golangci-lint v2 config (`.golangci.yml`). Active linters: bodyclose, errorlint, gocritic, misspell, nolintlint, revive, unconvert, unparam. Nolint directives require explanation and specific linter name. US English spelling enforced. Test files exclude bodyclose, goconst, unparam.

## Error Hierarchy

All errors in `internal/apperr/errors.go`. Base type `SchwabError` with typed subtypes:

| Error Type | Exit Code | When |
|---|---|---|
| AuthRequiredError | 3 | No token file found |
| AuthExpiredError | 3 | Token expired or 401 from API |
| AuthCallbackError | 3 | OAuth callback failed |
| OrderRejectedError | 5 | 400/422 on order placement |
| SymbolNotFoundError | 2 | Symbol not in API response or 404 |
| AccountNotFoundError | 2 | No account specified/found |
| HTTPError | 4 | Any other 4xx/5xx response |
| ValidationError | 1 | Input validation failures |
| OrderBuildError | 1 | Order construction failures |

Use `errors.As()` for type checking, never raw type assertions.

## Output Contract

All command output uses `internal/output` JSON envelopes:

- `WriteSuccess(w, data, meta)` - `{"data": ..., "metadata": ...}`
- `WriteError(w, err)` / `WriteCommandError(w, cmd, err)` - `{"error": ..., "exit_code": ..., "message": ...}` using the top-level `StructuredError` shape
- `WritePartial(w, data, errors, meta)` - `{"data": ..., "errors": [...], "metadata": ...}`
- Always `SetEscapeHTML(false)` on JSON encoders.

## CLI Structure

spf13/cobra. `buildApp()` in main.go constructs the command tree via `buildAppWithDeps()`. PersistentPreRunE on the root command skips auth for annotated Cobra commands such as `auth`, `symbol`, `completion`, `env-vars`, and `config-keys`, then loads config + token, refreshes if expired, populates `*client.Ref` for command access. PersistentPostRunE calls `Client.Close()` to release idle connections when the command finishes. Cobra-native flag error handlers wrap pflag parse failures in `apperr.FlagError`, and `internal/output` maps those errors to the stable `StructuredError` JSON contract. `env-vars` and `config-keys` are pure Cobra commands in `internal/commands/help_topics.go`. `RegisterOrderFlagAliasesOnTree(root)` walks the command tree to register flag aliases on qualifying order subcommands. The binary does not expose `--jsonschema`.

Command flags use project-owned struct tags (`flag`, `flagdescr`, `default`, `flagshort`) with `defineCobraFlags()` for Cobra/pflag registration. RunE handlers read the bound opts structs directly and call `validateCobraOptions()` when option structs expose `Validate(context.Context) []error`. Most order commands use `defineAndConstrain[O]()` from helpers.go, which wraps `defineCobraFlags()` + `MarkFlagsMutuallyExclusive`/`MarkFlagsOneRequired` in a single call. Root persistent flags (--account, --verbose, --config, --token) stay as manual Cobra registrations. The `--account`/`-a` value accepts an account hash directly, or resolves an account number through `AccountNumbers()` and a nickname through `UserPreference()`. Noun-only shorthand: `quote`, `history`, and `ta` parent commands accept positional symbol arguments that dispatch to their default subcommands (get, get, and dashboard respectively) via the `defaultSubcommand()` helper, enabling workflows like `schwab-agent quote AAPL` instead of `schwab-agent quote get AAPL`.

**Flag aliases**: Order subcommands that use `--action` and `--type` also accept `--instruction` and `--order-type` as aliases. This applies to 14 qualifying subcommands (place/preview/build equity/option/bracket/oco, place/preview option, replace equity, replace option). Aliases are registered post-setup by `RegisterOrderFlagAliasesOnTree()` with proper mutual exclusivity (`--action` vs `--instruction`, `--type` vs `--order-type`). Alias flags use explicit "alias for --" usage text so generated docs and help output make the relationship clear.

13 subcommands: auth (login/status/refresh), account (summary/list/get/numbers/set-default/transaction/resolve), position (list with --all-accounts/--account, local --symbol/--losers-only/--min-pnl/--max-pnl filters, and --sort pnl-desc/pnl-asc/value-desc), quote (get), order (list/get/place/preview/build/cancel/replace; place/build sub-types: equity/option/bracket/oco/buy-with-stop; replace has equity (parent RunE) and option subcommand), option (expirations/chain/contract), history (get; alias: price-history), instrument, market (hours/movers), symbol (build/parse), ta (sma/ema/rsi/macd/atr/bbands/stoch/adx/vwap/hv/expected-move), indicators (ta dashboard shortcut), analyze (quote + ta dashboard composite). Account summary is the token-efficient account picker for agents: it joins account hashes from account numbers with nicknames/primary/type from user preferences without fetching full balances. Account list/get enrich full results with nicknames from the preferences API (best-effort, degrades gracefully). Position list enriches with nicknames and adds computed cost basis / P&L fields before local filtering/sorting so `metadata.returned` reflects the emitted position count. Order list defaults to non-terminal statuses (use --status all for everything, or --recent for a 24-hour activity view that keeps terminal statuses). Option expirations lists available expiration dates with DTE and type. Option chain returns a compact sorted table of contracts with configurable field projection, DTE selection, and strike filtering. Option contract fetches a single contract with underlying quote and OCC symbol context. `quote get` supports structured option quoting via `--underlying`, `--expiration`, `--strike`, and `--call`/`--put` flags (mutually exclusive with positional symbol args). Order build supports practical multi-leg strategies including vertical, iron-condor, straddle, strangle, covered-call, collar, calendar, diagonal, butterfly, condor, vertical-roll, back-ratio, double-diagonal, and buy-with-stop. Quote, history, and ta commands support noun-only shorthand: positional symbol arguments dispatch to their default subcommands (get, get, and dashboard respectively).

## Config

JSON at `~/.config/schwab-agent/config.json`. Fields: `client_id`, `client_secret`, `callback_url`, `default_account`, `i-also-like-to-live-dangerously`. Env vars (`SCHWAB_CLIENT_ID`, `SCHWAB_CLIENT_SECRET`, `SCHWAB_CALLBACK_URL`) override file values. Default callback: `https://127.0.0.1:8182`.

## Safety Guards

- **Mutable operations** require `"i-also-like-to-live-dangerously": true` in config
- **Preview digest ledger**: `order preview --save-preview` stores the exact canonical order payload under the local state dir (`$SCHWAB_AGENT_STATE_DIR/previews`, `$XDG_STATE_HOME/schwab-agent/previews`, or `~/.local/state/schwab-agent/previews`) with `0700` directories and `0600` files. `order place --from-preview <digest>` reloads the account-bound payload, verifies the SHA-256 digest and 15-minute TTL, rejects account mismatches, and submits the stored payload unchanged.
- Market orders intentionally exclude price fields in the builder

## Testing Conventions

- Testify v1.11 (`require.*` for critical, `assert.*` for non-critical)
- `httptest.NewServer()` for API mocking with inline request validation
- Test helpers marked with `t.Helper()` (runTestCommand, testClient, jsonServer)
- Arrange/Act/Assert comment sections
- Table-driven subtests with `t.Run()`
- `t.TempDir()` for file I/O, no testdata/ directory
- `errors.As()` with `ErrorAs()` for typed error assertions
- Build tags (`//go:build taskNN`) for selective test execution
- CI: `go test -v -race -coverprofile=coverage.out ./...`

## Dependency Management

Renovate bot auto-merges patch/minor/digest after 7 days. Major versions require manual approval. Go toolchain updates grouped separately. Schedule: before 3am Monday, America/Chicago.

## Releases

Semver versioning. Releases are triggered by pushing a GPG-signed tag matching `v*` to `origin`.

### Process

1. Review commits since the last tag: `git log $(git describe --tags --abbrev=0)..HEAD --oneline`
2. Determine version bump (semver):
   - `feat:` commits = minor bump
   - Only `fix:`/`chore:`/`test:`/`docs:` = patch bump
   - Breaking changes = major bump
3. `make release VERSION=vX.Y.Z` (verifies main branch, clean tree, runs test+lint, generates tag message, creates GPG-signed tag)
4. Push the tag: `git push origin vX.Y.Z`

### Tag message format

`make release` auto-generates the tag message from conventional commits since the last tag:

```text
Release vX.Y.Z

Features:
- feat: commit messages grouped here

Fixes:
- fix: commit messages grouped here

Other:
- Everything else (chore, docs, test, ci, etc.)
```

Empty sections are omitted automatically.

### What happens after push

1. `.github/workflows/release.yml` triggers on the `v*` tag
2. goreleaser v2 builds binaries (Linux/Darwin, amd64/arm64, CGO disabled)
3. `ldflags` injects the version into the binary via `-X main.version={{.Version}}`
4. cosign signs the checksums file using keyless Sigstore/Fulcio OIDC (no private key)
5. goreleaser auto-generates changelog from conventional commits (all prefixes included)
## Intentional Design Decisions

1. **Shared client ref**: `client.Ref` (embedding `*Client`) is pre-allocated and shared by all commands; the Before hook populates `ref.Client` after auth
2. **Env vars override config**: Priority is env vars > config file > defaults
3. **Command references**: `SKILL.md` and `llms.txt` are maintained by hand alongside CLI changes, while runtime discovery uses Cobra help output. The binary does not expose `--jsonschema`.
4. **Workflow knowledge in help text**: Command Long descriptions and Example fields embed the workflow knowledge that was previously in separate skill files
5. **No testdata/**: All test data generated inline or via helper functions
6. **TLSConfig over HTTPClient**: `Config.TLSConfig()` returns a `*tls.Config` instead of the old `*http.Client` factory. Both the API client (`WithTLSConfig`) and schwab-go auth adapter HTTP client (`oauthHTTPClient`) use this to support insecure proxy setups. The OAuth callback server is delegated to schwab-go auth.
7. **Cobra tag migration pattern**: Command flags defined via project-owned struct tags (`flag`, `flagdescr`, `default`, `flagshort`) with `defineCobraFlags()` in command setup. RunE handlers read bound opts structs directly and call `validateCobraOptions()` for optional validation hooks. Root persistent flags and Cobra relationship checks stay as raw Cobra calls.
8. **Flag alias pattern**: `registerOrderFlagAliases()` adds `--instruction` (alias for `--action`) and `--order-type` (alias for `--type`) to qualifying order commands. `resolveOrderFlagAliasesViaFlags()` runs before handlers read option structs, copying alias values to canonical flags via `cmd.Flags().Set()`. `RegisterOrderFlagAliasesOnTree(root)` walks the full command tree post-setup to register aliases on all qualifying subcommands.
9. **Order replace option subcommand**: `order replace` parent retains its equity RunE. The `option` subcommand uses `buildOCCSymbol()` to construct the OCC symbol from `--underlying`, `--expiration`, `--strike`, `--call`/`--put` flags.
10. **Quote option mode**: `quote get` accepts structured option flags (`--underlying`, `--expiration`, `--strike`, `--call`/`--put`) as an alternative to positional symbol args. Uses `cobra.ArbitraryArgs` and detects option mode via `cmd.Flags().Changed()`.
11. **buy-with-stop dual-duration**: Entry leg uses user-specified duration (default DAY), exit legs are hardcoded to GTC. The builder creates a BracketParams copy with Duration=GoodTillCancel for exit builder calls, diverging from the bracket builder which uses one duration for all legs.
12. **resolveAccountDetailed pattern**: `resolveAccountDetailed()` is a parallel function to `resolveAccount()` that returns a `resolvedAccountInfo` struct with hash, account number, nickname, type, source ("explicit"/"default"/"preview"), and display label. `resolveAccount()` is unchanged (13 callers). Order commands use `resolveAccountDetailed()` to populate account context in `output.Metadata` (accountNickName, accountType, accountSource, accountDisplayLabel fields). Enrichment is best-effort: API failures return zero values and never block order placement. The `enrichAccountHash()` helper performs the AccountNumbers + UserPreference join for hash inputs.

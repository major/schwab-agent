# AGENTS.md - schwab-agent

> Keep this file and all subdirectory AGENTS.md files updated when the project changes.
> Keep README.md updated whenever the project changes.
> Check /usr/local for newer Go versions before assuming the system Go is current.
> Leave generous comments when fixing bugs or working around API quirks. Anything that might save a future developer from re-discovering the same issue is worth writing down.
> Keep skill files in `skills/` updated whenever CLI commands, flags, or behavior change.

## Project

Go CLI tool for AI agents to trade via Charles Schwab APIs. Single binary, JSON-first output, auto-generated skill files for agent consumption.

- **Module**: `github.com/major/schwab-agent`
- **Go version**: 1.26 (check `/usr/local/go/bin/go version` for newer installs)
- **Entry point**: `cmd/schwab-agent/main.go`
- **Dependencies**: urfave/cli/v3 (CLI framework), pkg/browser (OAuth flow), stretchr/testify (test assertions)

## Architecture

```text
cmd/schwab-agent/       Entry point, buildApp(), Before hook for auth
internal/
  auth/                 OAuth2 flow, token lifecycle, config (JSON + env vars)
  client/               Schwab API HTTP client (see internal/client/AGENTS.md)
  commands/             CLI command handlers (see internal/commands/AGENTS.md)
  apperr/               Typed error hierarchy with exit codes
  models/               Data structures/schemas for API payloads
  orderbuilder/         Order construction/validation (equity, option, bracket, OCO) + OCC symbol build/parse
  output/               JSON envelope writers (success, error, partial)
skills/                 Agent skill files (plain markdown, manually maintained)
```

## Build and Test

```bash
make build       # /usr/local/go/bin/go build -o schwab-agent ./cmd/schwab-agent/
make test        # go test -v ./...
make lint        # golangci-lint run ./...
make install     # Install to /usr/local/bin
make clean       # Remove binary
make release VERSION=vX.Y.Z  # Run test+lint, generate tag message, create GPG-signed tag
```

CI runs lint (golangci-lint v2.11) and test (race detector + coverage + build verification) on push to main and PRs. Releases via goreleaser on v* tags (Linux/Darwin, amd64/arm64, CGO disabled).

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
- `WriteError(w, code, message, details)` - `{"error": {"code": ..., "message": ..., "details": ...}}`
- `WritePartial(w, data, errors, meta)` - `{"data": ..., "errors": [...], "metadata": ...}`
- Always `SetEscapeHTML(false)` on JSON encoders.

## CLI Structure

urfave/cli v3. `buildApp()` in main.go constructs the command tree. Before hook skips auth for `auth`, `skills`, `schema`, and `symbol` commands, then loads config + token, refreshes if expired, populates `*client.Ref` for command access.

11 subcommands: auth (setup/login/status), account (list/get/numbers/set-default/transaction), position (list with --all-accounts/--account), quote (get), order (list/get/place/preview/build/cancel/replace; place/build sub-types: equity/option/bracket/oco), chain, history, instrument, market (hours/movers), symbol (build/parse), schema. Account list/get enriches results with nicknames from the preferences API (best-effort, degrades gracefully). Position list enriches with nicknames and adds computed cost basis / P&L fields. Order list defaults to non-terminal statuses (use --status all for everything).

## Config

JSON at `~/.config/schwab-agent/config.json`. Fields: `client_id`, `client_secret`, `callback_url`, `default_account`, `i-also-like-to-live-dangerously`. Env vars (`SCHWAB_CLIENT_ID`, `SCHWAB_CLIENT_SECRET`, `SCHWAB_CALLBACK_URL`) override file values. Default callback: `https://127.0.0.1:8182`.

## Safety Guards

- **Mutable operations** require `"i-also-like-to-live-dangerously": true` in config
- **Order placement/cancel/replace** require `--confirm` flag
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
3. **Schema introspection**: `schema` command auto-generates from CLI definitions, not manually maintained
4. **Skill files as plain markdown**: Skill files live in `skills/` as plain `.md` files, not generated from Go code
5. **No testdata/**: All test data generated inline or via helper functions

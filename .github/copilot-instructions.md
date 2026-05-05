# schwab-agent review instructions

Review this repository as a Go CLI for AI agents that trade through Charles Schwab APIs. The command output contract is JSON-first, safety checks protect real brokerage accounts, and workflow knowledge belongs in command help text plus generated agent docs.

Focus on bugs, security, data loss, broken command contracts, and project conventions. Do not nitpick formatting or style that golangci-lint already handles.

## Project invariants

- Use typed errors from `internal/apperr` and check them with `errors.As()`.
- Command output must go through `output.WriteSuccess`, `output.WriteError`, or `output.WritePartial`.
- JSON encoders must call `SetEscapeHTML(false)`.
- Mutable account operations require the explicit safety config flag and the preview digest flow where applicable.
- Market orders must not include price fields.
- Config values come from JSON files with environment variable overrides.
- Keep command help text, `README.md`, `SKILL.md`, and `llms.txt` aligned when command behavior changes.

## Security and trading safety

- Flag any credential, token, account hash, or secret exposure in logs, errors, tests, docs, or generated output.
- Verify order placement, cancellation, replacement, and preview flows keep account-bound safety checks intact.
- Check preview digest handling for exact payload reuse, SHA-256 digest validation, file permissions, account mismatches, and TTL enforcement.
- Treat silent fallback behavior around auth, token files, and API errors as risky unless it returns a clear structured error or documented degradation.

## Testing expectations

- Use `testify/require` for assertions that must stop a test and `testify/assert` for non-critical checks.
- Use `httptest.NewServer()` for HTTP API mocks with inline request validation.
- Mark test helpers with `t.Helper()`.
- Prefer table-driven subtests with `t.Run()`.
- Use `t.TempDir()` for file I/O. Do not introduce a `testdata/` directory.
- Validate typed errors with `assert.ErrorAs()` or `require.ErrorAs()`.

## Build and lint expectations

- CI runs `go test -v -race -coverprofile=coverage.out ./...`, build verification, smoke tests, govulncheck, CodeQL, and golangci-lint v2.
- Nolint directives require a specific linter name and an explanation.
- US English spelling is enforced.

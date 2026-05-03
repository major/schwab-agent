# Contributing to schwab-agent

Thanks for your interest in contributing. This document covers the basics for getting a PR merged.

## Getting started

### Prerequisites

- Go 1.25+ (check with `go version`; newer installs may live at `/usr/local/go/bin/go`)
- [golangci-lint](https://golangci-lint.run/) v2.11+
- A Schwab developer account is only needed if you're testing against the live API

### Setup

```bash
git clone https://github.com/major/schwab-agent.git
cd schwab-agent
make build
make test
make lint
```

All three should pass before you start making changes.

## Development workflow

### Branch and commit

1. Fork the repo and create a branch from `main`.
2. Use [Conventional Commits](https://www.conventionalcommits.org/) for all commit messages:
   - `feat: add support for trailing stops`
   - `fix: handle empty option chain response`
   - `test: add coverage for bracket order validation`
   - `docs: update skill file for chain command`
3. Keep commits focused. One logical change per commit.

### Build, test, lint

```bash
make build      # Build the binary
make test       # Run tests with race detector and coverage
make lint       # Run golangci-lint
```

CI runs the same checks with `go test -v -race -coverprofile=coverage.out ./...` and golangci-lint. Your PR won't merge if either fails.

### Code style

The project uses golangci-lint v2 with these active linters: bodyclose, errorlint, gocritic, gosec, misspell, nolintlint, revive, unconvert, unparam.

Key rules:

- **No `//nolint` without explanation**: Every nolint directive needs a reason and a specific linter name.
- **US English spelling**: Enforced by misspell.
- **Error wrapping**: Use `%w` for error wrapping, `errors.As()` for type checks. Never use raw type assertions on errors.

### Testing conventions

- Use [testify](https://github.com/stretchr/testify) for assertions: `require.*` for things that should stop the test, `assert.*` for things that shouldn't.
- Mock HTTP calls with `httptest.NewServer()` and validate requests inline.
- Use `t.TempDir()` for any file I/O. No `testdata/` directory.
- Mark test helpers with `t.Helper()`.
- Table-driven subtests with `t.Run()` are preferred.
- Arrange/Act/Assert comment sections help readability.

### Error handling

All errors live in `internal/apperr/errors.go` as typed subtypes of `SchwabError`. Each type maps to a specific exit code. If you're adding a new error scenario, check whether an existing type fits before creating a new one.

### Output format

All commands return JSON through `internal/output`. Use `WriteSuccess`, `WriteError`, or `WritePartial` - never write JSON directly to stdout. Always use `SetEscapeHTML(false)` on JSON encoders.

### Safety

This tool handles real money. One safety mechanism exists for mutable operations (order placement, cancellation, replacement):

1. Config file must have `"i-also-like-to-live-dangerously": true`

If you're adding a new command that modifies account state, it must use this guard.

## Pull requests

- Open a PR against `main`.
- Keep the description short: what changed and why. Link any related issues.
- CI must pass (lint + tests with race detector).
- Expect review from a [code owner](CODEOWNERS).

## Reporting bugs

Open a GitHub issue with:
- What you expected vs. what happened
- The command you ran (redact any account info)
- Relevant error output

## Security vulnerabilities

Do not open a public issue. Use [GitHub's security advisory feature](https://github.com/major/schwab-agent/security/advisories/new) instead. See [SECURITY.md](SECURITY.md) for details.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

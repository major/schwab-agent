---
applyTo: "**/*.go"
---

# Go review instructions

- Prefer small, focused functions with explicit error handling and clear return values.
- Use `%w` when wrapping errors with `fmt.Errorf`.
- Use `errors.As()` for typed error checks. Do not use raw type assertions for project errors.
- Avoid `init()` functions. Setup belongs in `main()` or command construction hooks.
- Preserve context propagation for API calls and cancellation.
- Keep exported identifiers documented with useful Go comments.
- Do not suggest style-only changes that `gofmt` or golangci-lint already enforces.

## CLI patterns

- Command factories should follow the existing `FooCommand(ref *client.Ref, w io.Writer) *cli.Command` style when applicable.
- Struct-tag flags should use structcli `Define()` during setup and `Unmarshal()` at the top of `RunE`.
- Root persistent flags and Cobra relationship checks may remain raw Cobra calls.
- Noun-only shorthand commands must dispatch to their documented default subcommands without bypassing auth behavior.

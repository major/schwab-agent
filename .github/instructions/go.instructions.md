---
applyTo: "**/*.go"
---

# Go review instructions

## Flag these (bugs, safety, correctness)

- Discarded errors assigned to `_` that silently hide failures.
- Missing or incorrect error wrapping (must use `%w` with `fmt.Errorf`).
- Raw type assertions on project errors instead of `errors.As()`.
- `init()` functions. Setup belongs in `main()` or command construction hooks.
- Broken context propagation for API calls and cancellation.
- `math/rand` used where `crypto/rand` is needed for keys or tokens.
- `panic` used for normal error handling instead of error returns.
- Unclosed response bodies or resources missing `defer` cleanup.

## Do not flag

- Comment style, grammar, punctuation, or missing doc comments.
- Variable, function, or receiver naming preferences.
- Import ordering or grouping.
- Line length, wrapping, or whitespace style.
- `any` vs `interface{}` usage.
- File organization or function ordering within a file.
- Suggesting code structure refactors (splitting functions, extracting helpers, reordering logic).
- Error message wording when the message already conveys the failure.
- Naked returns or named result parameter style.
- String concatenation style (`+` vs `fmt.Sprintf` vs `strings.Builder`).
- Adding type annotations, comments, or documentation that the author omitted.

golangci-lint, gofmt, and goimports handle formatting and style for this project. Do not duplicate their job.

## CLI patterns

- Command factories should follow the existing `NewFooCmd(ref *client.Ref, w io.Writer) *cobra.Command` style when applicable.
- Struct-tag flags should use project-owned struct tags (`flag`, `flagdescr`, `default`, `flagshort`) with `defineCobraFlags()` during setup and read bound option structs directly in `RunE`.
- Root persistent flags and Cobra relationship checks may remain raw Cobra calls.
- Noun-only shorthand commands must dispatch to their documented default subcommands without bypassing auth behavior.

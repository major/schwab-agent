---
applyTo: "**/*_test.go"
---

# Test review instructions

- Use `testify/assert` and `testify/require`, not bare `if` checks for assertions.
- Mock HTTP with `httptest.NewServer()` and validate expected request method, path, query, headers, and body inline.
- Mark reusable helpers with `t.Helper()`.
- Prefer table-driven subtests with `t.Run()`.
- Use `t.TempDir()` for file I/O.
- Keep generated data inline unless there is a clear reason to introduce fixtures.
- Do not request coverage-only tests when critical behavior is already covered.

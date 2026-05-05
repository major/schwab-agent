---
applyTo: "internal/client/**/*.go"
---

# Client review instructions

- `client.Ref` embeds `*Client`; it is pre-allocated for commands and populated after auth completes.
- HTTP errors should map to typed project errors with clear remediation context.
- Response bodies and idle connections must be closed where required.
- Preserve context propagation for cancellation and request timeouts.

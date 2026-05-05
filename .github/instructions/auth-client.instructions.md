---
applyTo: "internal/auth/**/*.go"
---

# Auth review instructions

- Verify credentials and tokens are never logged, returned in structured errors, or exposed in test output.
- Config loading must preserve the precedence of environment variables over JSON config values and defaults.
- Token file handling should degrade gracefully on permission and missing-file errors when that behavior is documented.
- OAuth token exchange and refresh flows use resty v3 and must respect configured TLS behavior.

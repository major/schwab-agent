---
applyTo: "internal/models/**/*.go"
---

# Model review instructions

- Model structs should match Schwab API JSON field names and avoid silently dropping important response fields.
- Changes to request payload structs need tests or smoke coverage that proves the emitted JSON shape.
- Avoid adding speculative fields unless Schwab API behavior or fixtures show they are needed.

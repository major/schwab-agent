---
applyTo: "internal/orderbuilder/**/*.go"
---

# Order builder review instructions

- Order builders must validate equity, option, bracket, OCO, and multi-leg strategies without accepting incomplete or contradictory inputs.
- OCC symbol build and parse logic must follow standard OCC formatting and preserve expiration, strike, and call or put semantics.
- Market orders intentionally exclude price fields.
- Mutable trading flows must retain safety guards and must not create shortcuts around preview digest checks.

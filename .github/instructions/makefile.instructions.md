---
applyTo: "Makefile"
---

# Makefile review instructions

- Makefile targets should have matching `.PHONY` declarations when they are not real files.
- Avoid adding flags that are already defaults.
- Keep task names aligned with `AGENTS.md`, `README.md`, and CI workflow usage.

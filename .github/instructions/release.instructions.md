---
applyTo: ".goreleaser.yml"
---

# Release review instructions

- Release automation must keep GoReleaser v2 behavior, CGO-disabled builds, version injection, and keyless cosign signing intact.
- Platform matrices should remain limited to the supported Linux and Darwin amd64 and arm64 targets unless project support changes.

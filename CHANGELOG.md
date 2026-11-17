# Changelog

## Unreleased

- Added native Linux arm64 release builds and offline package validation,
  starting with the first release after v0.1.0. Existing v0.1.0 artifacts
  are unchanged.

## v0.1.0 (2026-11-03)

- Python/Pyvider provider is now the sole maintained implementation.
- Two managed resources, five read-only data sources, and compatible state
  identities for existing router configuration.
- Hardware-tested Archer AX23 v2.0 read, update, import, replacement planning
  and delete lifecycles, plus RSA public-key verification.
- Explicit session-expiry handling with no retry on ambiguous transport errors.
- New read-only Python inventory generator for OpenTofu adoption.
- Frozen Linux/macOS/Windows provider releases with accompanying source,
  license attribution, SHA256 checksums, and GPG signing.
- GPL-3.0-or-later becomes the repository license. Go source, its CI and
  its inventory tool are retired, with earlier MIT-licensed revisions retained
  in Git history.


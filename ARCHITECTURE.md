# Provider architecture

Decision date: 2026-11-02. Status: accepted.

## Current implementation

The sole implementation is a Python provider using **Pyvider 0.8.1** for
Terraform Plugin Protocol 6. Its router backend is a pinned revision of the
GPL-3.0-or-later `tplinkrouterc6u` SG client. It supports the Archer AX23
v2.0 on firmware 1.1.2 Build 20250814. The provider implements two managed
resources and five data sources. It does not implement a separate wire
protocol, process sidecar or Go fallback.

The provider serializes access to one authenticated router session, performs
read-back verification after writes, and distinguishes explicit router
session expiry from uncertain network errors. Optional RSA public-key pins
reject substituted authentication keys before login.

The CLI is frozen with PyInstaller separately on Linux amd64, macOS arm64 and
Windows amd64. Native release archives are tested via offline Terraform
installation and schema introspection. The source release includes dependency
sources and attribution. NixOS requires a compatible glibc loader (for
example `nix-ld`).

## Why Go was retired

The earlier native Go client was a useful independent protocol verification
and supported initial router adoption. However, maintaining a separate SG
crypto implementation, the equivalent resource schema, and fixes already
present in the Python router library duplicated work. The Python version was
verified on the target router for reads, DHCP reservation and port-forward
lifecycles, and RSA key-pinning. It can be shipped without requiring users to
install Python or a separate service. The repository therefore retired its
Go client, Go tools and Go CI before the first public release.

The old Go commits remain in Git history under their original MIT license.
The current distribution is **GPL-3.0-or-later**, consistent with its linked
router library. Previous MIT grants are unaffected.

## State and operational constraints

- DHCP reservation identity is the normalized MAC address.
- Port-forward identity is the rule name.
- The Python inventory CLI generates Terraform variable and import files
  without changing router configuration.
- The router permits only one administrative session and does not offer
  transactional multi-row writes.
- Router state must be reconciled after ambiguous write outcomes. Do not
  replay network timeouts.
- The verified implementation is restricted to AX23 v2.0 until other hardware
  and firmware revisions receive actual tests.

See [PROTOCOL.md](PROTOCOL.md) for historical protocol observations and
[RELEASING.md](RELEASING.md) for release verification requirements.

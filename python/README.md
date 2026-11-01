# Python provider (the v0.1.0 release implementation)

This is the provider being promoted to v0.1.0: a Python/Pyvider implementation
with full schema parity and hardware-verified reads and writes against the
**Archer AX23 v2.0 (firmware 1.1.2 Build 20250814)** — the only advertised
target. The Go implementation on main remains the reference client until this
passes the final release checks, after which it is retired.

It uses Pyvider 0.8.1 for Terraform protocol v6 and the reviewed tplinkrouterc6u fork tag `pyprovider-pin-20261031`
for the TP-Link SG router protocol. No sidecar or duplicated router crypto.

Current proof:
- full seven-resource/data-source schema parity with the Go provider
- `tplink_dhcp_reservation` and `tplink_port_forward` CRUD adapters
- read-only `tplink_firmware`, `tplink_router`, `tplink_devices`,
  `tplink_dhcp_reservations`, and `tplink_port_forwards`
- Terraform plan/apply/import/replacement/drift/destroy against a CI-only
  persistent mock router, without physical router changes
- Terraform schema handshake for the Python development wrapper and frozen
  Linux binary; packaged Windows/macOS/Linux schema tests have passed in CI
- the pinned fork tag adds hardware-tested in-place DHCP updates; replace
  the Git dependency with a released library once upstream accepts it

Hardware verification status: full read parity (five data sources, compared
field-by-field against the Go provider) and a complete write lifecycle
(create/update/import/destroy of disposable rows) executed on the AX23 v2;
session-expiry re-authentication covered by unit tests. NixOS frozen-executable
execution requires `programs.nix-ld.enable = true;` — the only runtime
requirement. Remaining release work: cross-platform CI packaging checks and
the release-signing pipeline (`RELEASING.md`).

This prototype explicitly selects `TplinkRouterSG` instead of the legacy
auto-detect routine to avoid credential-probing across router families.
Switch to upstream `get_client_safe` when that API lands.

Run isolated unit tests:

```sh
python -m venv .venv
. .venv/bin/activate
pip install -e './python[test]'
pytest python/tests
```

Do **not** use the main repository's existing GoReleaser pipeline to package
this experiment. Any final Python registry package must be a self-contained
single executable tested through the full provider installation lifecycle.

## Licensing and distribution

The provider currently depends on a GPL-3.0-or-later Python library. The
experimental frozen executable should be treated as GPL-covered distribution
with the corresponding source and license notices. Do not publish it to a
registry as an MIT-only binary. Decide the repository license and provide
full dependency notices before a public release.

## RSA public-key pinning (v1 work in progress)

The provider schema exposes optional `password_key_pin` and `auth_key_pin`
attributes, corresponding to the Go provider's independently verified RSA
pre-authentication SPKI fingerprints. The pins must be enrolled over a trusted
connection. They are intentionally optional; configuring them against a Python
library without SG pin verification fails closed instead of silently ignoring
them.

The pinned library tag already includes both key checks. A separate,
unmerged [fork PR #3](https://github.com/repparw/TP-Link-Archer-C6U/pull/3)
contains an alternative implementation; it is **not** the code installed
by this provider. The tag and its reviewed commit are checked in CI against
`dependency-lock.json`.
**The pinned dependency tag already contains both SG pin verifiers.** Its
`test/test_sg_key_pins.py` tests are run against that exact tag in provider CI.
Hardware validation of correctly pinned login and mismatched keys is still
required before claiming the pinning feature has been validated on AX23.
No router I/O occurs during provider configuration.

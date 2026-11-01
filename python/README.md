# Experimental Python-only implementation

**Not production-ready.** This is an isolated architecture spike, not a replacement
for the validated Go provider on main and not a release target.

It uses Pyvider 0.8.1 for Terraform protocol v6 and a pinned tplinkrouterc6u fork commit
for the TP-Link SG router protocol. No sidecar or duplicated router crypto.

Current proof:
- full seven-resource/data-source schema parity with the Go provider
- `tplink_dhcp_reservation` and `tplink_port_forward` CRUD adapters
- read-only `tplink_firmware`, `tplink_router`, `tplink_devices`,
  `tplink_dhcp_reservations`, and `tplink_port_forwards`
- Terraform plan/apply/import/replacement/drift/destroy against a CI-only
  persistent mock router, without physical router changes
- Terraform schema handshake for the Python development wrapper and frozen
  Linux binary; packaged Windows/macOS tests are in progress
- a pinned fork commit adds missing in-place DHCP updates upstream; replace
  the Git dependency with a released library once upstream accepts it

Still requires real-device verification: exact firmware API behavior, handling
login expiry, NixOS executability, cancellation, and concurrency under load.
Cross-platform packaging and full Terraform/OpenTofu installs must pass before
considering any replacement of the Go provider.

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

The SG client change is staged in
[the Python library fork](https://github.com/repparw/TP-Link-Archer-C6U/pull/3).
**Current dependency tag does not yet contain the SG pinning implementation.**
Repin to a reviewed new immutable tag, then test correct/mismatched pins
end-to-end before treating the feature as finished. No router I/O occurs during
provider configuration.

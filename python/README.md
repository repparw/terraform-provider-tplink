# Experimental Python-only implementation

**Not production-ready.** This is an isolated architecture spike, not a replacement
for the validated Go provider on main and not a release target.

It uses Pyvider 0.8.1 for Terraform protocol v6 and tplinkrouterc6u 5.38.0
for the TP-Link SG router protocol. No sidecar or duplicated router crypto.

Current proof:
- provider schema and single-session serialized Python client
- read-only `tplink_firmware` data source
- `tplink_port_forward` resource with readback verification
- mock-only adapter tests

Not yet proven: actual Terraform/OpenTofu `init/plan/apply/import`, Pyvider
packaged executable, state conversion, replacement semantics, cancellation,
concurrent operations, Windows/macOS packaging, and parity with all Go data
sources. The Python library lacks in-place DHCP reservation updates, so
`tplink_dhcp_reservation` has deliberately not been ported. Import and
plan-modifier semantics need verification before any live write.

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

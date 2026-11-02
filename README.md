# terraform-provider-tplink

Terraform and OpenTofu provider for the **TP-Link Archer AX23 v2.0**,
verified with firmware **1.1.2 Build 20250814**. Other models and firmware
versions have not been validated on hardware.

The provider is implemented in **Python/Pyvider** using a pinned revision of
the GPL-licensed `tplinkrouterc6u` library. Releases package frozen executables
for Linux, macOS and Windows. End users do not need a Python installation.

The earlier Go implementation was retired before the first public release.
It remains available in Git history under its original MIT license.

## Features

- Data sources: `tplink_firmware`, `tplink_router`, `tplink_devices`,
  `tplink_dhcp_reservations`, `tplink_port_forwards`.
- Managed resources: `tplink_dhcp_reservation` (identified by MAC address)
  and `tplink_port_forward` (identified by rule name).
- Import, drift detection, read-back verification, and replacement when a
  resource's identity changes.
- Optional RSA public-key pinning via `password_key_pin` and `auth_key_pin`.

## Installation

**v0.1.0 has not been published yet.** Do not assume either registry source
is available until a signed release and registry registration are complete.

When registered, a configuration can use:

```hcl
terraform {
  required_providers {
    tplink = {
      source = "registry.opentofu.org/repparw/tplink"
      version = "~> 0.1"
    }
  }
}

variable "router_password" {
  type      = string
  sensitive = true
}

provider "tplink" {
  host     = "http://192.168.0.1"
  password = var.router_password
}
```

Use `registry.terraform.io/repparw/tplink` when publishing through
Terraform Registry instead. The two registries are registered separately.
The frozen Linux executable requires a compatible glibc runtime. NixOS may
need `programs.nix-ld.enable = true;` and suitable dynamic libraries.

See [RELEASING.md](RELEASING.md) for the full release procedure and
[docs/](docs/) for provider, resource and data-source schemas.

## Development

```sh
python -m venv .venv
. .venv/bin/activate
pip install -e './python[test]'
pytest -q python/tests
```

Frozen builds, registry package checks and an offline Terraform lifecycle
test run in GitHub Actions. See [python/README.md](python/README.md).

### Read-only router inventory

The old `cmd/tfinventory` helper is replaced by
`python -m tplink_py.inventory`. It produces
`inventory.auto.tfvars` and `imports.tf` for the router's existing DHCP
reservations and port forwards. Both files contain private LAN information.

```sh
export TPLINK_PASSWORD='...'
python -m tplink_py.inventory --dry-run --dir /path/to/nix/infra/router
python -m tplink_py.inventory --dir /path/to/nix/infra/router
```

The command connects to the router for reads only. It can optionally verify
`TPLINK_PASSWORD_KEY_PIN` and `TPLINK_AUTH_KEY_PIN`. Check generated files,
plan output, and existing OpenTofu state before any apply, especially after
out-of-band household changes.

## Router safety

The router accepts only one active admin session, so connecting the provider
can evict an interactive router UI session. Repeated incorrect logins can
trigger a long lockout. The provider does not retry failed credentials.

The verified AX23 firmware exposes its management interface over HTTP.
Use it only on a trusted local network and enroll RSA key pins over a trusted
connection. The encrypted application payload does not make HTTP equivalent
to TLS.

Terraform serializes router actions within one provider process, but it
cannot prevent external changes from another router client. A network timeout
may occur after a write succeeds. The provider only reauthenticates and
retries when the router explicitly rejects an expired session.

## References

- [Provider implementation and tests](python/)
- [Router SG protocol findings](PROTOCOL.md)
- [Architecture and migration decision](ARCHITECTURE.md)
- [OpenTofu router inventory](https://github.com/repparw/nix/tree/main/infra/router)

## License

**GPL-3.0-or-later** for the active repository and Python provider. The
frozen distribution includes GPL notices and matching dependency source
archives. The retired Go source remains available in Git history under
its earlier MIT terms. See [LICENSE](LICENSE), [python/NOTICE.md](python/NOTICE.md)
and [RELEASING.md](RELEASING.md).

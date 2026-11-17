# TP-Link provider for Terraform and OpenTofu

Manage TP-Link router configuration with Terraform or OpenTofu. The provider
supports DHCP reservations, port-forwarding rules, and read-only router
information. It has been hardware-tested on the **Archer AX23 v2.0** with
firmware **1.1.2 Build 20250814**.

**v0.1.0** is available on [GitHub Releases](https://github.com/repparw/terraform-provider-tplink/releases/tag/v0.1.0),
the [Terraform Registry](https://registry.terraform.io/providers/repparw/tplink/latest),
and the [OpenTofu Registry](https://search.opentofu.org/provider/repparw/tplink/latest).

## Quick start

Add the following to your OpenTofu configuration to read the router's model
without changing its settings:

```hcl
terraform {
  required_providers {
    tplink = {
      source  = "registry.opentofu.org/repparw/tplink"
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

data "tplink_firmware" "current" {}

output "router_model" {
  value = data.tplink_firmware.current.model
}
```

Provide the password outside the configuration, then initialize and inspect
the plan:

```sh
export TF_VAR_router_password='...'
tofu init
tofu plan
```

For **Terraform**, change the provider source to
`registry.terraform.io/repparw/tplink` and use `terraform init` /
`terraform plan`. The two registries are registered separately. To switch
between them, change the provider source address and re-run `init`.

## Supported functionality

- **Managed resources:** [DHCP reservations](docs/resources/dhcp_reservation.md)
  and [port-forwarding rules](docs/resources/port_forward.md).
- **Data sources:** [firmware](docs/data-sources/firmware.md),
  [router status](docs/data-sources/router.md),
  [connected devices](docs/data-sources/devices.md),
  [DHCP reservations](docs/data-sources/dhcp_reservations.md), and
  [port forwards](docs/data-sources/port_forwards.md).
- **State handling:** import existing entries, detect drift, verify writes,
  and replace resources when their identity changes.
- **Optional security:** pin the router's password and authentication RSA
  public keys with `password_key_pin` and `auth_key_pin`.

See the [provider schema](docs/index.md) and
[configuration examples](examples/). Import existing router entries instead
of recreating them.

## Compatibility

Only the Archer AX23 v2.0 running firmware 1.1.2 Build 20250814 has been
hardware-validated. Other models and firmware versions have not been tested.

v0.1.0 provides standalone executables for **Linux amd64**, **macOS arm64**,
and **Windows amd64**. Linux arm64 builds are not part of v0.1.0 and will ship
starting with the next release.

The provider uses Python/Pyvider and the GPL-licensed `tplinkrouterc6u`
library, but published executables do not require a Python installation.
Linux executables need a compatible glibc runtime. On NixOS,
`programs.nix-ld.enable = true;` and required libraries such as `libz`
may be needed.

## Safety and security

The verified AX23 firmware exposes its management interface over HTTP. Use
the provider on a trusted local network. The router allows only one active
admin session: a provider read or write can evict a web-UI session, and
repeated incorrect passwords may trigger a long lockout. The provider does
not retry failed credentials.

A transport timeout can occur *after* a router write has succeeded. The
provider retries only when the router explicitly reports session expiry,
not after ambiguous network errors. Review plans before applying changes,
particularly for existing network rules.

Optional RSA key pins protect against substituted login keys. Enroll them
over a trusted connection using the read-only
[`keyprobe`](python/README.md#rsa-public-key-pinning) utility; encrypted
router application payloads do not make HTTP equivalent to TLS.

## Development and utilities

To run tests or use the read-only inventory utility, clone this repository
and install the Python project (Python 3.11+):

```sh
python -m venv .venv
. .venv/bin/activate
pip install -e './python[test]'
pytest -q python/tests
```

To generate `inventory.auto.tfvars` and `imports.tf` for existing DHCP
reservations and port forwards:

```sh
export TPLINK_PASSWORD='...'
python -m tplink_py.inventory --dry-run --dir /path/to/config
python -m tplink_py.inventory --dir /path/to/config
```

The inventory utility reads router state without modifying it. Generated
files contain private LAN information and must stay out of version control.
It can optionally verify `TPLINK_PASSWORD_KEY_PIN` and
`TPLINK_AUTH_KEY_PIN`. Review import declarations and the plan before
applying. See [python/README.md](python/README.md) for development details
and [RELEASING.md](RELEASING.md) for release procedures.

## License

The active provider and repository are **GPL-3.0-or-later**. The former Go
implementation and its inventory tool were retired before v0.1.0 and
remain available in Git history under their original MIT terms. See
[LICENSE](LICENSE), [python/NOTICE.md](python/NOTICE.md), and the
[release source archives](https://github.com/repparw/terraform-provider-tplink/releases/tag/v0.1.0).

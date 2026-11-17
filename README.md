# TP-Link provider for Terraform and OpenTofu

Manage DHCP reservations and port-forwarding rules on TP-Link routers with
Terraform or OpenTofu. The provider is **hardware-tested on the Archer AX23 v2.0**
with firmware **1.1.2 Build 20250814**. Other models and firmware versions have
not been validated.

The provider is implemented in Python with Pyvider and the GPL-licensed
`tplinkrouterc6u` library. Published provider packages are standalone
executables; installing Python is not required to use them.

## Features

- Manage [DHCP reservations](docs/resources/dhcp_reservation.md) and
  [port-forwarding rules](docs/resources/port_forward.md).
- Read router information, firmware, connected devices, DHCP reservations,
  and port forwards through five [data sources](docs/data-sources/).
- Import existing entries, detect drift, verify writes, and replace resources
  when their identity changes.
- Optionally pin the router's password and authentication RSA public keys
  using `password_key_pin` and `auth_key_pin`.

## Installation

**v0.1.0** is published on [GitHub Releases](https://github.com/repparw/terraform-provider-tplink/releases/tag/v0.1.0)
and listed in both the
[OpenTofu Registry](https://search.opentofu.org/provider/repparw/tplink/latest)
and [Terraform Registry](https://registry.terraform.io/providers/repparw/tplink/latest).

For OpenTofu:

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
```

Set the password outside the configuration, then initialize and review a plan:

```sh
export TF_VAR_router_password='...'
tofu init
tofu plan
```

For **Terraform**, change the provider source to
`registry.terraform.io/repparw/tplink` and use `terraform init` /
`terraform plan`. Registry namespaces are independent; a Terraform lock
file will not automatically use the OpenTofu registry address.

v0.1.0 provides frozen binaries for **Linux amd64**, **macOS arm64**, and
**Windows amd64**. Linux arm64 builds were added to the release workflow
*after* v0.1.0 and will be available in a subsequent release. Linux binaries
require a compatible glibc runtime. On NixOS, `programs.nix-ld.enable = true;`
and required libraries (such as `libz`) may be needed.

See the [provider schema](docs/index.md),
[resource examples](examples/resources/), and
[data-source examples](examples/data-sources/).
Existing router entries should be **imported** rather than recreated.

## Router safety

The verified AX23 firmware exposes its management interface over HTTP. Use
the provider on a trusted local network. The router allows only one active
admin session: a provider read or write can evict a web-UI session, and
repeated incorrect passwords may trigger a long lockout.

A transport timeout can occur *after* a router write has succeeded. The
provider retries only when the router explicitly reports session expiry,
not after ambiguous network errors. Review plans before applying changes,
particularly for existing household network rules.

Optional RSA key pins protect against substituted login keys. Enroll them
over a trusted connection using the read-only
[`keyprobe`](python/README.md#rsa-public-key-pinning) utility; the encrypted
router application payload does not make HTTP equivalent to TLS.

## Development and inventory

For source development and the read-only inventory utility, use Python 3.11+
and install the project from a clone:

```sh
python -m venv .venv
. .venv/bin/activate
pip install -e './python[test]'
pytest -q python/tests
```

To generate `inventory.auto.tfvars` and `imports.tf` for existing router
reservations and port forwards:

```sh
export TPLINK_PASSWORD='...'
python -m tplink_py.inventory --dry-run --dir /path/to/config
python -m tplink_py.inventory --dir /path/to/config
```

The inventory command reads router state without modifying it. Its output
contains private LAN information, so keep generated files out of version
control. It can optionally check `TPLINK_PASSWORD_KEY_PIN` and
`TPLINK_AUTH_KEY_PIN`. Review the generated import declarations and plan
before applying.

The former Go implementation and its inventory tool were retired before
v0.1.0 and remain available in Git history. For build and release details
see [python/README.md](python/README.md) and [RELEASING.md](RELEASING.md).

## License

The active Python provider and repository are **GPL-3.0-or-later**; the
retired Go implementation retains its original MIT terms in Git history.
See [LICENSE](LICENSE), [python/NOTICE.md](python/NOTICE.md), and the
[release source archives](https://github.com/repparw/terraform-provider-tplink/releases/tag/v0.1.0).

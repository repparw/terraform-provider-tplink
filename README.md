# terraform-provider-tplink

Terraform / OpenTofu provider for the **TP-Link Archer AX23 v2.0,
firmware 1.1.2 Build 20250814**. Other hardware and firmware versions are
not yet validated.

The intended v0.1.0 release is implemented in [Python/Pyvider](python/)
and distributed as frozen executables, so end users do not need Python.
The previous [Go implementation](main.go) remains an MIT-licensed reference
client until retirement. The Python release is GPL-3.0-or-later due to its
router-library dependency. See [PROTOCOL.md](PROTOCOL.md) for the verified
wire protocol and [RELEASING.md](RELEASING.md) for publication steps.

## Requirements

- Terraform >= 1.6 / OpenTofu >= 1.7
- Network access to the router's HTTP management interface
  (this firmware generation serves local management over plain HTTP; HTTPS
  access is blocked by the router itself)

## Provider installation (development)

```hcl
terraform {
  required_providers {
    tplink = {
      source = "registry.opentofu.org/repparw/tplink"
    }
  }
}

provider "tplink" {
  host     = "http://192.168.0.1"
  password = var.router_password
}
```

To test the Go reference client locally (not the v0.1.0 Python release),
build and install via a plugin directory (verified with OpenTofu 1.13):

```sh
go build -o terraform-provider-tplink .
mkdir -p ~/.tofu.d/plugins/registry.opentofu.org/repparw/tplink/0.1.0/linux_amd64
cp terraform-provider-tplink ~/.tofu.d/plugins/registry.opentofu.org/repparw/tplink/0.1.0/linux_amd64/
tofu init -plugin-dir=~/.tofu.d/plugins
```

`terraform` works the same way with `~/.terraform.d/plugins` and
`terraform init -plugin-dir=...`. The Python release uses the signed
registry ZIPs described in [RELEASING.md](RELEASING.md). Neither registry
address should be assumed published until registration is complete.

## Data sources (read-only)

| Data source | Description |
| --- | --- |
| `tplink_firmware` | Model, hardware revision, firmware version |
| `tplink_router` | WAN/LAN status, uptime, CPU/memory, client counts |
| `tplink_devices` | Connected-device inventory (wired, host, guest, IoT) |
| `tplink_dhcp_reservations` | All static DHCP assignments |
| `tplink_port_forwards` | All port-forwarding (virtual server) rules |

```hcl
data "tplink_firmware" "current" {}
output "firmware" { value = data.tplink_firmware.current }

data "tplink_port_forwards" "all" {}
output "forwarded" { value = data.tplink_port_forwards.all.rules }
```

See [examples/](examples/) for complete configurations.

## Resources

### `tplink_dhcp_reservation`

A static DHCP address assignment, identified by the device **MAC address**
(import ID: the MAC). The provider accepts colon, dash or Cisco-dotted MACs,
preserving the configured spelling in Terraform state. The router identity and
resource ID are normalized.

```hcl
resource "tplink_dhcp_reservation" "printer" {
  mac      = "AA-BB-CC-DD-EE-FF"
  ip       = "192.168.0.50"
  hostname = "printer"
  enabled  = true
}
```

### `tplink_port_forward`

A port forwarding (virtual server) rule, identified by its **name**
(import ID: the name).

```hcl
resource "tplink_port_forward" "web" {
  name          = "web"
  ip            = "192.168.0.4"
  external_port = "443"
  internal_port = "443"
  protocol      = "TCP"
  enabled       = true
}
```

Import existing configuration:

```sh
tofu import tplink_port_forward.web "https"
tofu import tplink_dhcp_reservation.pi "2C-CF-67-00-4F-47"
```

## Operational notes (read before applying)

- **Single admin session.** The router allows exactly one admin login;
  provider runs evict an interactive web-UI session and vice versa. The Python
  provider retries only explicit session-expiry errors; uncertain network
  timeouts are surfaced rather than replaying possible writes.
- **Lockout.** Roughly 12 failed logins trigger a ~2 h lockout. The provider
  never retries failed logins. Keep the password in TF_VAR_router_password,
  a secret manager, or an interactive prompt — never in git.
- **HTTP-only management.** The login flow travels over plain HTTP by
  design of this firmware; the password's confidentiality rests on the
  RSA key exchange. Do not expose the management interface to untrusted
  LAN segments.
- **Write verification.** Every create/update reads the row back and checks
  all known planned values, not just the identity.
- **Concurrency.** Multi-request table changes are serialized within the provider
  process. Other processes or a web-UI session can still race with an apply.
- **Cancellation.** The Go reference client propagates Terraform cancellation
  to router HTTP requests. Do not assume equivalent cancellation behavior in
  the Python client without separate verification.
- **Verified hardware.** The full managed-resource write cycle (create →
  update → destroy of a disposable disabled reservation and port forward,
  read-back verified after each step) and all read operations were executed
  live through OpenTofu against the AX23 v2 on 2026-10-29, with the original
  configuration verified intact afterwards. The protocol handshake is
  additionally covered by the mock-router conformance test suite.

## Development

For the Python release candidate:

```sh
python -m venv .venv
. .venv/bin/activate
pip install -e './python[test]'
pytest -q python/tests
```

For the separate Go reference implementation:

```sh
go test ./...
go run ./cmd/livescan  # read-only; requires authorized router access
gofmt -l . && go vet ./...
```

## License

The Python provider and frozen distribution are **GPL-3.0-or-later**.
The separate Go reference implementation remains **MIT**.
See [python/NOTICE.md](python/NOTICE.md) and [RELEASING.md](RELEASING.md)
for third-party source and license distribution requirements.

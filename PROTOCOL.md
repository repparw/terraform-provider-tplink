# SG-hardened TP-Link router protocol

Verified on **Archer AX23 v2.0**, firmware **1.1.2 Build 20250814**
(web UI build 1.11.0, certification profile `SG CLS L1 STAGE2`).
Initial read-only investigation: **2026-10-25**; Go client and live writes
verified **2026-10-26–29**.

This document owns the reusable protocol findings. For the current Python/Pyvider implementation decision, see [ARCHITECTURE.md](ARCHITECTURE.md).
For router-specific declarative configuration, state and backups, see
[`repparw/nix/infra/router`](https://github.com/repparw/nix/tree/main/infra/router).

## Scope and compatibility

The client implements the new-generation **SG-hardened** LuCI web API seen on
the verified AX23 v2. Other models with similar UI/authentication are plausible
compatibility candidates, **not independently tested** by this project.
Older classic and newer simplified firmware authentication modes are not
supported; firmware upgrades should be followed by a read-only check and a
`tofu plan` before any write.

The router's UI JavaScript exposed roughly **148 API forms** during the original
investigation. This is **not** the provider's supported API surface: the current provider intentionally exposes only the methods required for its read-only
data sources and two managed resources. Do not assume the presence of a
discovered router endpoint makes it writable via this provider.

## Authentication and transport

- API endpoint: form POST requests under
  `/cgi-bin/luci/;stok=<token>/<path>?form=<name>`.
- `login?form=keys` provides RSA password-encryption public key material;
  `login?form=auth` provides sequence and key-exchange material.
- The login flow uses a SHA-256 digest of username + password, RSA PKCS#1
  v1.5 for the password, RSA-OAEP for the session-key signature, and
  AES-128-CBC for the encrypted payload.
- Authenticated requests use AES-128-CBC envelopes and chunked HMAC-SHA256
  signatures; the request signature incorporates the hash of that request's
  encrypted body and its sequence.
- The session is identified by the `stok` URL token and `sysauth` cookie.
  **Only one admin session** can be active at a time; provider activity can
  sign out an interactive browser session.
- This AX23 firmware exposes local management over **HTTP**, not usable HTTPS.
  The application-layer encrypted envelope does **not** provide TLS transport
  security. Keep the management interface off untrusted segments.
- Roughly twelve failed logins can trigger a long lockout. The provider must
  never blindly retry a bad password. Expired sessions and user-conflict
  responses are handled separately from failed authentication.

The Python provider uses a pinned revision of the `tplinkrouterc6u` SG client.
The former Go mock router and its protocol tests remain accessible through Git
history for independent reference.

## Implemented forms

| Purpose | LuCI path | Operation |
| --- | --- | --- |
| Firmware identification | `admin/firmware?form=upgrade` | `read` |
| Router status and connected clients | `admin/status?form=all` | `read` |
| Static DHCP reservations | `admin/dhcps?form=reservation` | `load`, `insert`, `update`, `remove` |
| IPv4 port-forward rules (virtual servers) | `admin/nat?form=vs` | `load`, `insert`, `update`, `remove` |
| Logout | `admin/system?form=logout` | `write` |

Table loads may return a bare JSON array (observed on AX23 v2),
`{"data": [...]}` or `{"list": [...]}`, and `{}` for an empty table.
Malformed nonempty payloads should fail rather than silently produce an
empty inventory.

Table writes use form-encoded parameters:

- **Insert:** `operation=insert`, `new=<JSON row>`.
- **Update:** `operation=update`, `key=<current identity>`,
  `new=<JSON row>`, `old=<JSON row>` (and row index where used).
- **Remove:** `operation=remove`, `key=<current identity>`,
  `index=<zero-based position>`.

Reservations are identified by **MAC address** and port forwards by
**rule name**. Operations using a table index require loading the current
table before choosing an index. Router firmware may accept unknown write
parameters without reporting an error, so the provider **reads back every
create/update** and checks the resulting values. The provider serializes table
mutations within a provider process; concurrent management outside it can
invalidate row positions, so table writes are not transactional across clients.
The mock router models remove as a key-and-index check and rejects a stale
index instead of deleting an unrelated row. The regression test demonstrates
the client's behavior *under that model*, not the real firmware's atomicity.
Although key and index are both required by the observed API, their exact
validation semantics under concurrent external changes have **not** been
verified against hardware. Do not treat the mock as a concurrency guarantee.
The provider deliberately avoids automatic retries on ambiguous write failures;
refresh and re-plan before trying again.

## Verified results and limitations

- **2026-10-25:** read-only Python SG-client probe confirmed firmware/status,
  12 static reservations, 2 port forwards, DHCP leases, Wi-Fi information,
  and additional diagnostic endpoints. The original probe exposed internal
  SSIDs, hostnames, local IPs, and MAC addresses despite redacting passwords
  and WAN addresses. Its output is private network inventory, not suitable
  for publication. Private router inventory must never be included in a public repository.
  The Python read-only inventory command is `python -m tplink_py.inventory`.
- **2026-10-26:** native Go client login and read operations verified on the
  AX23 v2.
- **2026-10-29:** separately authorized, *disabled*, disposable reservation
  and port-forward create/update/delete operations were verified through
  OpenTofu with read-back. All original rows were confirmed intact after
  cleanup. The existing 12 reservations and 2 forwards were imported into
  OpenTofu with a clean subsequent plan.
- The upstream Python port-forward implementation was separately verified on
  the same model in
  [`TP-Link-Archer-C6U#248`](https://github.com/AlexandrErohin/TP-Link-Archer-C6U/pull/248).
  The Python router library and current provider are GPL-3.0-or-later covered.
  The retired Go client remains available under its original MIT license in Git history.
- **Not verified:** Wi-Fi writes, other firmware generations, broad endpoint
  CRUD beyond the two resources, or atomic rollback on multi-row failures.

The research was derived from the router's own web UI JavaScript, checked
against independently implemented SG clients and hardware behavior. See
[ARCHITECTURE.md](ARCHITECTURE.md) for the implementation rationale and
[README.md](README.md) for supported provider interfaces.

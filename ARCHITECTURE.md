# Architecture decision: native Go client vs Python sidecar

Date: 2026-10-26. Status: accepted.

## Context

This provider manages TP-Link home routers on the new-generation (SG-hardened)
web UI — verified against an Archer AX23 v2.0, firmware 1.1.2 Build 20250814
(SG_L1_S2 certification). The protocol is documented in
[`repparw/nix` research note](https://github.com/repparw/nix/blob/main/docs/research/archer-ax23-api-opentofu.md)
and upstream PR
[AlexandrErohin/TP-Link-Archer-C6U#248](https://github.com/AlexandrErohin/TP-Link-Archer-C6U/pull/248).

Two implementation shapes were compared:

| | A. Native Go protocol implementation | B. Go provider + Python sidecar (wrapping `tplinkrouterc6u`) |
| --- | --- | --- |
| Deployment | Single static binary, no runtime dependencies | Requires Python 3 + `tplinkrouterc6u` installed and reachable; either spawned by the provider (fragile lifecycle inside Terraform, PATH/venv management on CI and workstations) or run out-of-band (breaks `tofu plan/apply` self-containment) |
| Session handling | Full control: one session per run, re-auth on `timeout`, mutex serialization | Session state lives in the sidecar process; Terraform cannot own its lifecycle cleanly |
| Latency | Direct, ~50-100 ms per call | Extra hop + process management per operation |
| Crypto correctness risk | Must port SHA256/RSA-v1.5/OAEP/AES-CBC/HMAC-SHA256 exactly (mitigated: byte-level mock-router conformance test + live verification; upstream library's SG client serves as reference) | Inherits the battle-tested Python crypto for free |
| Upstream fix inheritance | Not automatic — protocol changes require porting | Automatic via `pip` upgrades |
| Maintenance | Protocol surface we implement is small (login + ~10 forms); firmware generations beyond SG can be added behind the same client interface | Track upstream's ~25 client families, but only through its API |
| Licensing | Clean: protocol reimplementation from public documentation; provider stays MIT/Apache-2.0, no GPL obligations | `tplinkrouterc6u` is **GPL-3.0**: a provider that *links* it would have to be GPL-3.0; a separate-process sidecar avoids linking but ships and depends on GPL code at runtime |
| Registry publishing | Straightforward (goreleaser, static binary) | Awkward (sidecar distribution not covered by the Terraform/OpenTofu provider protocol) |

## Decision

**Option A — native Go.** The decisive factors are deployment simplicity
(Terraform/OpenTofu providers must be single binaries to work well across
workstations, CI, and the registry) and licensing (GPL-3.0 avoidance without
process-boundary tricks). The porting risk is contained because:

1. The wire format is fully documented from the router's own JS bundles and
   cross-checked against three independent implementations (the upstream
   Python library's SG client, the `archer-router-mcp` byte-exact auth-flow
   spec, and live packet behavior of the AX23 v2).
2. `internal/client/mockrouter_test.go` implements the **router's side** of
   the protocol (real RSA keypairs, OAEP signature decryption, HMAC
   verification, AES envelope) so the client is tested against an independent
   implementation of the wire format, not against itself.
3. `cmd/livescan` performs a read-only live check against real hardware and
   was run successfully against the AX23 v2 (2026-10-26: login, firmware,
   status, devices, 12 DHCP reservations, 2 port forwards).

## Non-gotheses / consequences

- Other firmware generations (classic MD5-era c6u, simplified V1_11, or the
  512-bit-auth variants) are **not** implemented; the client fails with a
  clear error if the handshake does not look like SG. Adding a generation
  means a new auth strategy behind the same `Client` interface.
- When upstream ships crypto fixes for future firmware, they must be ported;
  the upstream PR remains the reference implementation.
- The provider keeps one router session per Terraform run (the router allows
  exactly one admin session) and re-authenticates transparently on
  `timeout`/`user conflict` responses. It never retries failed logins
  (≈12 failures trigger a ~2 h lockout).
- Writes always read back after the fact (the router accepts unknown params
  silently), which doubles as drift detection on the next refresh.

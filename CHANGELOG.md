# Changelog

## [0.1.0] - 2026-10-27

- Initial MVP: native Go client for the SG-hardened TP-Link protocol
  (Archer AX23 v2.0 fw 1.1.2 verified).
- Data sources: tplink_firmware, tplink_router, tplink_devices,
  tplink_dhcp_reservations, tplink_port_forwards.
- Resources: tplink_dhcp_reservation, tplink_port_forward (import,
  read-back-after-write drift handling).
- Protocol-conformance test suite with a mock router implementing the
  SG handshake server-side.

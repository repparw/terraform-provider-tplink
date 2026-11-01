"""Managed DHCP reservation using upstream Python library's in-place updater."""
from attrs import define
from ipaddress import IPv4Address
from macaddress import EUI48
from pyvider.resources import BaseResource, ResourceContext, register_resource
from pyvider.schema import PvsSchema, a_bool, a_str, a_unknown, s_resource

from tplink_py.common import provider


def normalized_mac(mac):
    """Strict, canonical MAC, unlike the library's soft-fallback get_mac()."""
    return str(EUI48(mac))


@define
class DHCPConfig:
    mac: str
    ip: str
    hostname: str = ""
    enabled: bool = True


@define
class DHCPState:
    id: str
    mac: str
    ip: str
    hostname: str
    enabled: bool


def select_reservation(rows, mac):
    matches = [r for r in rows if normalized_mac(r.macaddr) == normalized_mac(mac)]
    if len(matches) > 1:
        raise RuntimeError("Duplicate DHCP reservation for the same MAC; refusing write")
    return matches[0] if matches else None


def state_from_reservation(row, configured_mac=None):
    return DHCPState(
        id=normalized_mac(row.macaddr),
        mac=configured_mac if configured_mac is not None else normalized_mac(row.macaddr),
        ip=row.ipaddr,
        hostname=row.hostname or "",
        enabled=row.enabled,
    )


async def verified_reservation(mac, expected=None, configured_mac=None):
    rows = await provider().call("get_ipv4_reservations")
    match = select_reservation(rows, mac)
    if expected is not None:
        if match is None:
            raise RuntimeError("DHCP reservation missing after write")
        if (match.ipaddr != expected.ip
                or (expected.hostname is not None and
                    (match.hostname or "") != expected.hostname)
                or match.enabled != expected.enabled):
            raise RuntimeError("Router returned DHCP fields different from the plan")
    return state_from_reservation(match, configured_mac) if match else None


@register_resource("tplink_dhcp_reservation")
class DHCPReservation(BaseResource):
    config_class = DHCPConfig
    state_class = DHCPState

    @classmethod
    def get_schema(cls) -> PvsSchema:
        return s_resource({
            "id": a_str(computed=True),
            "mac": a_str(required=True),
            "ip": a_str(required=True),
            "hostname": a_str(optional=True, computed=True),
            "enabled": a_bool(optional=True, computed=True, default=True),
        })

    async def _validate_config(self, config: DHCPConfig) -> list[str]:
        errors = []
        try:
            normalized_mac(config.mac)
        except (ValueError, TypeError):
            errors.append("Invalid MAC address")
        try:
            IPv4Address(config.ip)
        except (ValueError, TypeError):
            errors.append("Invalid IPv4 address")
        return errors

    async def _update(self, ctx, base_plan):
        # Changing the spelling of the same MAC must not delete a reservation.
        # Require replacement only if the normalized identity changes.
        # Formatting-only edits converge in state at apply time (the apply
        # stores the config spelling without touching the router); the plan
        # must keep the config value verbatim because `mac` is required and
        # non-computed — rewriting it here makes OpenTofu reject the plan.
        if (ctx.state is not None and ctx.config is not None
                and normalized_mac(ctx.config.mac)
                != normalized_mac(ctx.state.mac)):
            ctx.require_replace("mac")
        return base_plan, None

    async def _create(self, ctx, base_plan):
        base_plan["id"] = a_unknown(a_str())
        return base_plan, None

    async def _create_apply(self, ctx: ResourceContext):
        cfg = ctx.config
        if await verified_reservation(cfg.mac):
            raise RuntimeError("Existing DHCP reservation must be imported")
        await provider().call(
            "add_ipv4_reservation", cfg.mac, cfg.ip,
            comment=cfg.hostname or "", enable=cfg.enabled)
        return await verified_reservation(cfg.mac, cfg, cfg.mac), None

    async def read(self, ctx: ResourceContext):
        if ctx.state is None:
            return None
        return await verified_reservation(ctx.state.mac,
                                          configured_mac=ctx.state.mac)

    async def _update_apply(self, ctx: ResourceContext):
        cfg = ctx.config
        if ctx.state is None:
            raise RuntimeError("Missing previous DHCP reservation state")
        if normalized_mac(cfg.mac) != normalized_mac(ctx.state.mac):
            raise RuntimeError("MAC identity changed; destroy/recreate required")
        # A MAC punctuation/case-only change can update Terraform state
        # without writing an unchanged row back to the router.
        if (cfg.ip == getattr(ctx.state, "ip", None)
                and cfg.hostname == getattr(ctx.state, "hostname", None)
                and cfg.enabled == getattr(ctx.state, "enabled", None)):
            return DHCPState(normalized_mac(cfg.mac), cfg.mac, cfg.ip,
                             cfg.hostname, cfg.enabled), None
        await provider().call(
            "update_ipv4_reservation", cfg.mac, ipaddr=cfg.ip,
            comment=cfg.hostname, enable=cfg.enabled)
        return await verified_reservation(cfg.mac, cfg, cfg.mac), None

    async def _delete_apply(self, ctx: ResourceContext):
        if ctx.state is not None and await verified_reservation(ctx.state.mac):
            await provider().call("delete_ipv4_reservation", ctx.state.mac)
            if await verified_reservation(ctx.state.mac):
                raise RuntimeError("DHCP reservation remained after deletion")

    async def import_state(self, ctx: ResourceContext, import_id: str):
        mac = normalized_mac(import_id)
        state = await verified_reservation(mac, configured_mac=mac)
        if state is None:
            raise RuntimeError("DHCP reservation to import does not exist")
        return state

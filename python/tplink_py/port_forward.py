"""Experimental port-forward resource backed by tplinkrouterc6u."""
from attrs import define
from pyvider.resources import BaseResource, ResourceContext, register_resource
from pyvider.schema import PvsSchema, a_bool, a_str, a_unknown, s_resource

from tplink_py.common import matching_forward, provider


@define
class PortForwardConfig:
    name: str
    ip: str
    external_port: str
    internal_port: str
    protocol: str
    enabled: bool = True


@define
class PortForwardState:
    id: str
    name: str
    ip: str
    external_port: str
    internal_port: str
    protocol: str
    enabled: bool


def state_from_rule(rule, configured_protocol=None):
    # Preserve the practitioner's protocol spelling on a case-insensitive
    # router. Terraform otherwise rejects a post-apply drift in configured
    # "tcp" vs the router's canonical "TCP".
    protocol = rule.protocol
    if configured_protocol and configured_protocol.upper() == rule.protocol.upper():
        protocol = configured_protocol
    return PortForwardState(
        id=rule.name,
        name=rule.name,
        ip=rule.ipaddr,
        external_port=rule.external_port,
        internal_port=rule.internal_port,
        protocol=protocol,
        enabled=rule.enabled,
    )


async def verified_forward(name, expected=None, configured_protocol=None):
    rules = await provider().call("get_ipv4_port_forwards")
    match = matching_forward(rules, name)
    if expected is not None:
        if match is None:
            raise RuntimeError("Port forward missing after router write")
        if not (
            match.ipaddr == expected.ip
            and match.external_port == expected.external_port
            and match.internal_port == expected.internal_port
            and match.protocol.upper() == expected.protocol.upper()
            and match.enabled == expected.enabled
        ):
            raise RuntimeError("Router returned values different from the plan")
    return state_from_rule(
        match, configured_protocol or (expected.protocol if expected else None)
    ) if match else None


@register_resource("tplink_port_forward")
class PortForward(BaseResource):
    config_class = PortForwardConfig
    state_class = PortForwardState

    @classmethod
    def get_schema(cls) -> PvsSchema:
        return s_resource({
            "id": a_str(computed=True),
            "name": a_str(required=True, requires_replace=True),
            "ip": a_str(required=True),
            "external_port": a_str(required=True),
            "internal_port": a_str(required=True),
            "protocol": a_str(required=True),
            "enabled": a_bool(optional=True, computed=True, default=True),
        })

    async def _validate_config(self, config: PortForwardConfig) -> list[str]:
        errors = []
        if not config.name.strip():
            errors.append("port forward name is required")
        if config.protocol.upper() not in ("TCP", "UDP", "ALL"):
            errors.append("protocol must be TCP, UDP or ALL")
        return errors

    async def _create(self, ctx, base_plan):
        base_plan["id"] = a_unknown(a_str())
        return base_plan, None

    async def _create_apply(self, ctx: ResourceContext):
        cfg = ctx.config
        if await verified_forward(cfg.name):
            raise RuntimeError(
                "Existing port forward must be imported instead of created")
        await provider().call(
            "add_ipv4_port_forward",
            cfg.name, cfg.external_port, cfg.internal_port,
            cfg.ip, protocol=cfg.protocol, enable=cfg.enabled)
        return await verified_forward(cfg.name, cfg), None

    async def read(self, ctx: ResourceContext):
        if ctx.state is None:
            return None
        return await verified_forward(
            ctx.state.name, configured_protocol=ctx.state.protocol)

    async def _update_apply(self, ctx: ResourceContext):
        cfg = ctx.config
        if not ctx.state or cfg.name != ctx.state.name:
            raise RuntimeError(
                "Changing a port-forward name requires replacement")
        await provider().call(
            "update_ipv4_port_forward", cfg.name,
            external_port=cfg.external_port,
            internal_port=cfg.internal_port,
            ipaddr=cfg.ip, protocol=cfg.protocol, enable=cfg.enabled)
        return await verified_forward(cfg.name, cfg), None

    async def import_state(self, ctx: ResourceContext, import_id: str):
        state = await verified_forward(import_id)
        if state is None:
            raise RuntimeError("Port forward to import does not exist")
        return state

    async def _delete_apply(self, ctx: ResourceContext):
        if ctx.state and await verified_forward(ctx.state.name):
            await provider().call("delete_ipv4_port_forward", ctx.state.name)
            if await verified_forward(ctx.state.name):
                raise RuntimeError("Port forward remained after deletion")

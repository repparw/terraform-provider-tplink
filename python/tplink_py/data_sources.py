"""Read-only data sources matching the Go provider's names and fields."""
from attrs import define, field
from pyvider.data_sources import BaseDataSource, register_data_source
from pyvider.resources.context import ResourceContext
from pyvider.schema import (
    PvsSchema, a_bool, a_num, a_obj, a_set, a_str, s_data_source,
)

from tplink_py.common import provider


@define
class NoInputs:
    pass


@define
class RouterState:
    conn_type: str
    wan_ip: str
    wan_gateway: str
    wan_mac: str
    wan_uptime: int
    lan_ip: str
    lan_mac: str
    lan_dhcp_enabled: bool
    cpu_usage: float
    mem_usage: float
    devices_total: int
    wired_total: int
    wireless_total: int
    guest_wireless_total: int
    iot_wireless_total: int


@register_data_source("tplink_router")
class Router(BaseDataSource):
    config_class = NoInputs
    state_class = RouterState

    @classmethod
    def get_schema(cls) -> PvsSchema:
        return s_data_source({
            "conn_type": a_str(computed=True),
            "wan_ip": a_str(computed=True),
            "wan_gateway": a_str(computed=True),
            "wan_mac": a_str(computed=True),
            "wan_uptime": a_num(computed=True),
            "lan_ip": a_str(computed=True),
            "lan_mac": a_str(computed=True),
            "lan_dhcp_enabled": a_bool(computed=True),
            "cpu_usage": a_num(computed=True),
            "mem_usage": a_num(computed=True),
            "devices_total": a_num(computed=True),
            "wired_total": a_num(computed=True),
            "wireless_total": a_num(computed=True),
            "guest_wireless_total": a_num(computed=True),
            "iot_wireless_total": a_num(computed=True),
        })

    async def _validate_config(self, config: NoInputs) -> list[str]:
        return []

    async def read(self, ctx: ResourceContext) -> RouterState:
        s = await provider().call("get_status")
        v4 = await provider().call("get_ipv4_status")
        return RouterState(
            conn_type=s.conn_type or "",
            wan_ip=s.wan_ipv4_addr or "",
            wan_gateway=s.wan_ipv4_gateway or "",
            wan_mac=s.wan_macaddr or "",
            wan_uptime=int(s.wan_ipv4_uptime or 0),
            lan_ip=s.lan_ipv4_addr or "",
            lan_mac=s.lan_macaddr or "",
            lan_dhcp_enabled=bool(v4.lan_ipv4_dhcp_enable),
            cpu_usage=float(s.cpu_usage or 0),
            mem_usage=float(s.mem_usage or 0),
            devices_total=int(s.clients_total or 0),
            wired_total=int(s.wired_total or 0),
            wireless_total=int(s.wifi_clients_total or 0),
            guest_wireless_total=int(s.guest_clients_total or 0),
            iot_wireless_total=int(s.iot_clients_total or 0),
        )


@define
class DevicesState:
    devices: list[dict] = field(factory=list)


@register_data_source("tplink_devices")
class Devices(BaseDataSource):
    config_class = NoInputs
    state_class = DevicesState

    @classmethod
    def get_schema(cls) -> PvsSchema:
        return s_data_source({
            "devices": a_set(a_obj({
                "hostname": a_str(), "ip": a_str(),
                "mac": a_str(), "type": a_str(),
            }), computed=True),
        })

    async def _validate_config(self, config: NoInputs) -> list[str]:
        return []

    async def read(self, ctx: ResourceContext) -> DevicesState:
        s = await provider().call("get_status")
        def segment(dev):
            typ = dev.type
            if typ.is_iot(): return "iot"
            if typ.is_guest_wifi(): return "guest"
            if typ.is_host_wifi(): return "wifi"
            if typ.value == "wired": return "wired"
            return "unknown"
        return DevicesState(devices=[
            {"hostname": d.hostname or "", "ip": d.ipaddr,
             "mac": d.macaddr, "type": segment(d)}
            for d in s.devices
        ])


@define
class ReservationsState:
    reservations: list[dict] = field(factory=list)


@register_data_source("tplink_dhcp_reservations")
class Reservations(BaseDataSource):
    config_class = NoInputs
    state_class = ReservationsState

    @classmethod
    def get_schema(cls) -> PvsSchema:
        return s_data_source({
            "reservations": a_set(a_obj({
                "hostname": a_str(), "ip": a_str(), "mac": a_str(),
                "enabled": a_bool(),
            }), computed=True),
        })

    async def _validate_config(self, config: NoInputs) -> list[str]:
        return []

    async def read(self, ctx: ResourceContext) -> ReservationsState:
        rows = await provider().call("get_ipv4_reservations")
        return ReservationsState(reservations=[
            {"hostname": r.hostname or "", "ip": r.ipaddr,
             "mac": r.macaddr, "enabled": r.enabled} for r in rows
        ])


@define
class ForwardsState:
    rules: list[dict] = field(factory=list)


@register_data_source("tplink_port_forwards")
class Forwards(BaseDataSource):
    config_class = NoInputs
    state_class = ForwardsState

    @classmethod
    def get_schema(cls) -> PvsSchema:
        return s_data_source({
            "rules": a_set(a_obj({
                "name": a_str(), "ip": a_str(), "external_port": a_str(),
                "internal_port": a_str(), "protocol": a_str(), "enabled": a_bool(),
            }), computed=True),
        })

    async def _validate_config(self, config: NoInputs) -> list[str]:
        return []

    async def read(self, ctx: ResourceContext) -> ForwardsState:
        rules = await provider().call("get_ipv4_port_forwards")
        return ForwardsState(rules=[
            {"name": r.name, "ip": r.ipaddr, "external_port": r.external_port,
             "internal_port": r.internal_port, "protocol": r.protocol,
             "enabled": r.enabled} for r in rules
        ])

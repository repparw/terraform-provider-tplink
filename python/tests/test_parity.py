"""No network access: resource lifecycle, import and data-source projections."""
import asyncio
from types import SimpleNamespace as Obj
from unittest.mock import AsyncMock, patch
from tplink_py.dhcp_reservation import (
    DHCPReservation, DHCPConfig, verified_reservation, normalized_mac,
)
from tplink_py.port_forward import PortForward, verified_forward
from tplink_py.data_sources import Router, Devices, Reservations, Forwards


def run(coro):
    return asyncio.run(coro)


def test_dhcp_normalizes_and_imports():
    r = Obj(macaddr="AA-BB-CC-DD-EE-FF", ipaddr="192.168.0.20",
            hostname="printer", enabled=True)
    fake = Obj(call=AsyncMock(return_value=[r]))
    with patch("tplink_py.dhcp_reservation.provider", return_value=fake):
        state = run(DHCPReservation().import_state(None, "aa:bb:cc:dd:ee:ff"))
        assert state.id == normalized_mac("aa:bb:cc:dd:ee:ff")
        assert state.mac == state.id
        assert state.ip == "192.168.0.20"
        assert state.hostname == "printer"


def test_dhcp_known_fields_verified():
    row = Obj(macaddr="AA-BB-CC-DD-EE-FF", ipaddr="192.168.0.20",
              hostname="actual", enabled=True)
    desired = DHCPConfig("AA-BB-CC-DD-EE-FF", "192.168.0.20", "desired", True)
    fake = Obj(call=AsyncMock(return_value=[row]))
    with patch("tplink_py.dhcp_reservation.provider", return_value=fake):
        try:
            run(verified_reservation(desired.mac, desired))
        except RuntimeError as err:
            assert "different" in str(err)
        else:
            raise AssertionError("mismatch accepted")


def test_dhcp_in_place_update():
    row = Obj(macaddr="AA-BB-CC-DD-EE-FF", ipaddr="192.168.0.21",
              hostname="updated", enabled=False)
    fake = Obj(call=AsyncMock(return_value=[row]))
    plan = DHCPConfig("aa:bb:cc:dd:ee:ff", "192.168.0.21", "updated", False)
    old = Obj(mac="AA-BB-CC-DD-EE-FF")
    with patch("tplink_py.dhcp_reservation.provider", return_value=fake):
        state, private = run(DHCPReservation()._update_apply(
            Obj(config=plan, state=old)))
    assert state.ip == plan.ip
    assert state.mac == plan.mac
    assert private is None
    assert fake.call.await_args_list[0].args[0] == "update_ipv4_reservation"
    assert fake.call.await_args_list[0].kwargs == {
        "ipaddr": plan.ip, "comment": plan.hostname, "enable": False
    }


def test_dhcp_change_of_identity_fails_before_write():
    plan = DHCPConfig("BB-BB-CC-DD-EE-FF", "192.168.0.21")
    ctx = Obj(config=plan, state=Obj(mac="AA-BB-CC-DD-EE-FF"))
    fake = Obj(call=AsyncMock())
    with patch("tplink_py.dhcp_reservation.provider", return_value=fake):
        try:
            run(DHCPReservation()._update_apply(ctx))
        except RuntimeError as err:
            assert "required" in str(err)
        else:
            raise AssertionError("MAC changes allowed without replacement")
    fake.call.assert_not_called()


def test_import_missing_forward_refused():
    fake = Obj(call=AsyncMock(return_value=[]))
    with patch("tplink_py.port_forward.provider", return_value=fake):
        try:
            run(PortForward().import_state(None, "nonexistent"))
        except RuntimeError as err:
            assert "does not exist" in str(err)
        else:
            raise AssertionError("phantom import accepted")


def test_router_data_projection():
    status = Obj(conn_type="dhcp", wan_ipv4_addr="192.0.2.2",
                 wan_ipv4_gateway="192.0.2.1", wan_macaddr="aa",
                 wan_ipv4_uptime=12, lan_ipv4_addr="192.168.0.1",
                 lan_macaddr="bb", cpu_usage=.2, mem_usage=.3,
                 clients_total=5, wired_total=1, wifi_clients_total=2,
                 guest_clients_total=1, iot_clients_total=1)
    fake = Obj(call=AsyncMock(side_effect=[status, Obj(lan_ipv4_dhcp_enable=True)]))
    with patch("tplink_py.data_sources.provider", return_value=fake):
        st = run(Router().read(None))
    assert st.lan_dhcp_enabled
    assert st.iot_wireless_total == 1
    assert st.wan_ip == "192.0.2.2"


def test_nested_data_sources_have_matching_keys():
    for cls in (Devices, Reservations, Forwards):
        schema = cls.get_schema().block.attributes
        assert len(schema) == 1
        assert next(iter(schema.values())).computed


def test_replacement_flags_set():
    assert DHCPReservation.get_schema().block.attributes["mac"].requires_replace
    assert PortForward.get_schema().block.attributes["name"].requires_replace


def test_no_login_on_configuration():
    from tplink_py import TPLinkProvider
    fake = Obj(authorize=lambda: None)
    with patch("tplink_py.TplinkRouterSG", return_value=fake):
        inst = TPLinkProvider()
        run(inst.configure(Obj(
            host="http://router", username="admin",
            password="secret", verify_ssl=True)))
    assert not inst._authenticated

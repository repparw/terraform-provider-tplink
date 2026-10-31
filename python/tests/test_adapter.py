"""Mock-only checks. Never connects to a live router."""
import asyncio
from types import SimpleNamespace as Obj
from unittest.mock import AsyncMock, patch

from tplink_py.common import matching_forward
from tplink_py.port_forward import (
    PortForward, PortForwardConfig, verified_forward,
)
from tplink_py.firmware import Firmware
from tplink_py import TPLinkProvider


def test_duplicate_names_fail_closed():
    rows = [Obj(name="dup"), Obj(name="dup")]
    try:
        matching_forward(rows, "dup")
    except RuntimeError as error:
        assert "duplicate" in str(error)
    else:
        raise AssertionError("ambiguous name was accepted")


def test_verify_every_field_after_create():
    rule = Obj(name="web", ipaddr="192.168.0.9",
               external_port="443", internal_port="8443",
               protocol="TCP", enabled=False)
    cfg = PortForwardConfig(
        "web", "192.168.0.9", "443", "8443", "tcp", False)
    fake = Obj(call=AsyncMock(return_value=[rule]))
    with patch("tplink_py.port_forward.provider", return_value=fake):
        got = asyncio.run(verified_forward("web", cfg))
    assert got.id == "web"
    assert got.enabled is False
    fake.call.assert_awaited_once_with("get_ipv4_port_forwards")


def test_mismatch_rejected():
    rule = Obj(name="web", ipaddr="192.168.0.9",
               external_port="443", internal_port="8443",
               protocol="TCP", enabled=True)
    cfg = PortForwardConfig(
        "web", "192.168.0.9", "443", "8443", "TCP", False)
    fake = Obj(call=AsyncMock(return_value=[rule]))
    with patch("tplink_py.port_forward.provider", return_value=fake):
        try:
            asyncio.run(verified_forward("web", cfg))
        except RuntimeError as error:
            assert "different" in str(error)
        else:
            raise AssertionError("silent router ignore accepted")


def test_update_preserves_identity():
    resource = PortForward()
    cfg = PortForwardConfig(
        "web", "192.168.0.9", "443", "8443", "TCP", True)
    state = Obj(name="different")
    ctx = Obj(config=cfg, state=state)
    try:
        asyncio.run(resource._update_apply(ctx))
    except RuntimeError as error:
        assert "replacement" in str(error)
    else:
        raise AssertionError("identity changed in place")


def test_components_are_instantiable():
    assert isinstance(PortForward(), PortForward)
    assert isinstance(Firmware(), Firmware)


def test_provider_schema_includes_router_config():
    instance = TPLinkProvider()
    asyncio.run(instance.setup())
    names = instance.schema().block.attributes.keys()
    assert {"host", "username", "password", "verify_ssl"} <= set(names)

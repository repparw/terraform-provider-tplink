"""Session-expiry reauthentication and error-path behavior."""
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from tplink_py import TPLinkProvider


@pytest.fixture
def provider_instance():
    p = TPLinkProvider()
    p.router = MagicMock()
    p.router.authorize = MagicMock()
    p._authenticated = False
    p.lock = __import__("asyncio").Lock()
    return p


def _router_with_timeout_then_success(method_name: str, result):
    """Router stub whose first `method_name` call raises a session timeout."""
    router = MagicMock()
    calls = {"n": 0}

    def call_method(*args, **kwargs):
        calls["n"] += 1
        if calls["n"] == 1:
            raise RuntimeError(
                "TplinkRouterSG - Response with error; errorcode timeout")
        return result

    setattr(router, method_name, MagicMock(side_effect=call_method))
    return router, calls


def test_reauthorizes_once_on_session_timeout(provider_instance):
    router, calls = _router_with_timeout_then_success("get_ipv4_reservations", [])
    provider_instance.router = router

    async def scenario():
        return await provider_instance.call("get_ipv4_reservations")

    import asyncio
    result = asyncio.run(scenario())

    assert result == []
    assert calls["n"] == 2                      # timed out once, retried once
    assert router.authorize.call_count == 2     # initial + re-auth
    assert provider_instance._authenticated is True


def test_does_not_retry_on_non_timeout_errors(provider_instance):
    router = MagicMock()
    router.get_firmware = MagicMock(
        side_effect=RuntimeError("connection refused"))
    router.authorize = MagicMock()
    provider_instance.router = router
    provider_instance._authenticated = True

    import asyncio
    with pytest.raises(RuntimeError, match="connection refused"):
        asyncio.run(provider_instance.call("get_firmware"))

    assert router.authorize.call_count == 0     # no re-auth attempt

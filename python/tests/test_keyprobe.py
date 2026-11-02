"""Verify the RSA key probe never authenticates or writes router settings."""
from unittest.mock import MagicMock

from tplink_py.keyprobe import probe


def test_read_only_key_probe():
    router = MagicMock()
    router._pwdNN = "A1"
    router._pwdEE = "010001"
    router._nn = "B2"
    router._ee = "010001"
    router._fingerprint_spki.side_effect = ["pwd-sha", "auth-sha"]
    factory = MagicMock(return_value=router)

    result = probe("http://router.invalid", 17, router_class=factory)

    assert result == {
        "password_key_pin": "pwd-sha",
        "auth_key_pin": "auth-sha",
    }
    factory.assert_called_once_with("http://router.invalid", "", timeout=17)
    router._request_pwd_keys.assert_called_once_with()
    router._request_auth_keys.assert_called_once_with()
    router.authorize.assert_not_called()
    router.request.assert_not_called()

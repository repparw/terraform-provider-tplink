"""Experimental Python-only TP-Link provider.

Register the Pyvider components on import. All real router calls use the
existing tplinkrouterc6u protocol implementation, never a second Go client.
"""
import asyncio
import inspect
import os

from pyvider.providers import BaseProvider, ProviderMetadata, register_provider
from pyvider.schema import PvsSchema, a_bool, a_str, s_provider
from tplinkrouterc6u import TplinkRouterSG

from tplink_py import firmware, port_forward  # noqa: F401


@register_provider("tplink")
class TPLinkProvider(BaseProvider):
    def __init__(self):
        super().__init__(
            metadata=ProviderMetadata(
                name="tplink", version=os.environ.get("PYVIDER_PROVIDER_VERSION", "0.0.0"), protocol_version="6"))
        self.router = None
        self.lock = asyncio.Lock()
        self._authenticated = False

    # Pyvider 0.8.x composes the schema from registered capabilities.
    # RouterCapability provides the host, username and password attributes.

    async def configure(self, config):
        await super().configure(config)
        # This POC intentionally selects SG explicitly until passive-first
        # discovery is upstream. Never call the legacy get_client() detector.
        # The pinned fork tag supports these pre-auth key checks. Keep the
        # guard for incompatible dependency changes and fail closed.
        pin_kwargs = {
            key: str(value) for key in ("password_key_pin", "auth_key_pin")
            if (value := getattr(config, key, None))
        }
        if pin_kwargs and not all(
            key in inspect.signature(TplinkRouterSG.__init__).parameters
            for key in pin_kwargs
        ):
            raise RuntimeError(
                "Configured RSA pins require the SG key-pinning library update. "
                "This provider will not disable pin verification silently."
            )
        router = TplinkRouterSG(
            config.host, config.password,
            username=config.username,
            verify_ssl=not bool(config.tls_skip_verify),
            timeout=int(config.timeout_seconds or 30),
            **pin_kwargs)
        # Do not evict a router GUI session merely for Terraform configure.
        # Authorize only when the first read/write actually needs the router.
        self.router = router
        self._authenticated = False

    async def _authorize(self):
        await asyncio.to_thread(self.router.authorize)
        self._authenticated = True

    async def call(self, method, *args, **kwargs):
        """Serialize use of the single router session across parallel resources."""
        async with self.lock:
            if self.router is None:
                raise RuntimeError("TP-Link provider has not been configured")
            if not self._authenticated:
                await self._authorize()
            try:
                return await asyncio.to_thread(
                    getattr(self.router, method), *args, **kwargs)
            except Exception as exc:
                # The router ends sessions on idle expiry or eviction by
                # another admin, answering HTTP 200 with errorcode "timeout":
                # the request was rejected before execution, so a single
                # re-authenticate-and-retry is safe (mirrors the Go client).
                if "timeout" not in str(exc).lower():
                    raise
                self._authenticated = False
                await self._authorize()
                return await asyncio.to_thread(
                    getattr(self.router, method), *args, **kwargs)

    async def close(self):
        if self.router is not None:
            try:
                if self._authenticated:
                    async with self.lock:
                        await asyncio.to_thread(self.router.logout)
            finally:
                self.router = None
                self._authenticated = False


def main():
    from pyvider.cli import main as pyvider_main
    pyvider_main()

"""Experimental Python-only TP-Link provider.

Register the Pyvider components on import. All real router calls use the
existing tplinkrouterc6u protocol implementation, never a second Go client.
"""
import asyncio

from pyvider.providers import BaseProvider, ProviderMetadata, register_provider
from pyvider.schema import PvsSchema, a_bool, a_str, s_provider
from tplinkrouterc6u import TplinkRouterSG

from tplink_py import firmware, port_forward  # noqa: F401


@register_provider("tplink")
class TPLinkProvider(BaseProvider):
    def __init__(self):
        super().__init__(
            metadata=ProviderMetadata(
                name="tplink", version="0.0.0", protocol_version="6"))
        self.router = None
        self.lock = asyncio.Lock()
        self._authenticated = False

    # Pyvider 0.8.x composes the schema from registered capabilities.
    # RouterCapability provides the host, username and password attributes.

    async def configure(self, config):
        await super().configure(config)
        # This POC intentionally selects SG explicitly until passive-first
        # discovery is upstream. Never call the legacy get_client() detector.
        router = TplinkRouterSG(
            config.host, config.password,
            username=config.username, verify_ssl=config.verify_ssl)
        # Do not evict a router GUI session merely for Terraform configure.
        # Authorize only when the first read/write actually needs the router.
        self.router = router
        self._authenticated = False

    async def call(self, method, *args, **kwargs):
        """Serialize use of the single router session across parallel resources."""
        async with self.lock:
            if self.router is None:
                raise RuntimeError("TP-Link provider has not been configured")
            if not self._authenticated:
                await asyncio.to_thread(self.router.authorize)
                self._authenticated = True
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

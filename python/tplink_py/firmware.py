"""Read-only firmware data source implemented with the upstream Python library."""
from attrs import define
from pyvider.data_sources import BaseDataSource, register_data_source
from pyvider.resources.context import ResourceContext
from pyvider.schema import PvsSchema, a_str, s_data_source

from tplink_py.common import provider


@define
class FirmwareConfig:
    pass


@define
class FirmwareState:
    model: str
    hardware_version: str
    firmware_version: str


@register_data_source("tplink_firmware")
class Firmware(BaseDataSource):
    config_class = FirmwareConfig
    state_class = FirmwareState

    @classmethod
    def get_schema(cls) -> PvsSchema:
        return s_data_source({
            "model": a_str(computed=True),
            "hardware_version": a_str(computed=True),
            "firmware_version": a_str(computed=True),
        })

    async def read(self, ctx: ResourceContext) -> FirmwareState:
        info = await provider().call("get_firmware")
        return FirmwareState(
            model=info.model,
            hardware_version=info.hardware_version,
            firmware_version=info.firmware_version,
        )

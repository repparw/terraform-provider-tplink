"""Provider configuration for current Pyvider's capability-composed schema."""
from pyvider.capabilities import BaseCapability, register_capability
from pyvider.schema import a_bool, a_num, a_str


@register_capability("tplink_router")
class RouterCapability(BaseCapability):
    def __init__(self, config=None):
        self.config = config

    @staticmethod
    def get_schema_contribution():
        return {
            "host": a_str(required=True, description="Router management URL"),
            "username": a_str(default="admin", optional=True),
            "password": a_str(required=True, sensitive=True),
            "tls_skip_verify": a_bool(default=False, optional=True),
            "timeout_seconds": a_num(default=30, optional=True),
        }

"""Frozen Python provider entrypoint (used only by packaging)."""
import os

# Frozen builds have no pyproject.toml beside them: declare the identity and
# protocol contract the same way pyproject.toml does for development runs.
os.environ.setdefault("PYVIDER_PROVIDER_NAME", "tplink")
os.environ.setdefault("SUPPORTED_PROTOCOL_VERSIONS", "6")

import tplink_py  # noqa: F401,E402 - registers provider components

from pyvider.cli.__main__ import main  # noqa: E402

if __name__ == "__main__":
    main()

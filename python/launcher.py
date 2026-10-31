"""Frozen Python provider entrypoint (used only by packaging)."""
import tplink_py  # noqa: F401 - registers provider components

from pyvider.cli.__main__ import main

if __name__ == "__main__":
    main()

"""Cross-platform PyInstaller bundle with a baked provider version.

No Python interpreter is needed at runtime on the target host.
"""
import os
from pathlib import Path
import re
import subprocess
import sys


def main():
    root = Path(__file__).resolve().parent.parent
    version = os.environ.get("PYVIDER_PROVIDER_VERSION", "0.0.0")
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?", version):
        raise ValueError(f"Invalid provider build version: {version!r}")
    runtime_hook = root / "dist" / "pyinstaller-runtime-version.py"
    runtime_hook.parent.mkdir(parents=True, exist_ok=True)
    # A PyInstaller runtime hook executes before the provider's launcher.
    # Force the build version instead of trusting the end user's environment.
    runtime_hook.write_text(
        "import os\n"
        f"os.environ['PYVIDER_PROVIDER_VERSION'] = {version!r}\n",
        encoding="utf-8",
    )
    args = [
        sys.executable, "-m", "PyInstaller",
        "--noconfirm", "--clean", "--onefile",
        "--name", "terraform-provider-tplink",
        "--distpath", str(root / "dist" / "python"),
        "--workpath", str(root / "dist" / "pyinstaller-work"),
        "--specpath", str(root / "dist" / "pyinstaller-spec"),
        "--runtime-hook", str(runtime_hook),
        "--collect-all", "pyvider",
        "--collect-all", "tplinkrouterc6u",
        "--collect-submodules", "provide",
        "--collect-submodules", "pyvider.rpcplugin",
        "--collect-submodules", "tplink_py",
        "--copy-metadata", "pyvider",
        "--copy-metadata", "tplinkrouterc6u",
        "--copy-metadata", "terraform-provider-tplink-python",
        str(root / "python" / "launcher.py"),
    ]
    subprocess.run(args, cwd=root, check=True)


if __name__ == "__main__":
    main()

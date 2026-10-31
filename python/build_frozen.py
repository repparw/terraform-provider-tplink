"""Cross-platform, reproducible PyInstaller invocation for the Python POC."""
from pathlib import Path
import subprocess
import sys


def main():
    root = Path(__file__).resolve().parent.parent
    args = [
        sys.executable, "-m", "PyInstaller",
        "--noconfirm", "--clean", "--onefile",
        "--name", "terraform-provider-tplink",
        "--distpath", str(root / "dist" / "python"),
        "--workpath", str(root / "dist" / "pyinstaller-work"),
        "--specpath", str(root / "dist" / "pyinstaller-spec"),
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

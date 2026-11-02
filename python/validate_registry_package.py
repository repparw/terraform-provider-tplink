"""Offline Terraform init from the actual versioned ZIP (no network or router)."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parent.parent
SOURCE = "registry.test.invalid/repparw/tplink"


def main(version: str, platform: str) -> None:
    terraform = shutil.which("terraform")
    if terraform is None:
        raise FileNotFoundError("terraform must be installed to check registry ZIPs")
    zip_file = ROOT / "dist" / "release" / (
        f"terraform-provider-tplink_{version}_{platform}.zip"
    )
    if not zip_file.is_file():
        raise FileNotFoundError(zip_file)
    with tempfile.TemporaryDirectory(prefix="tplink-offline-install-") as temp:
        directory = Path(temp)
        mirror = directory / "mirror"
        mirror_parent = mirror / "registry.test.invalid" / "repparw" / "tplink"
        mirror_parent.mkdir(parents=True)
        shutil.copy(zip_file, mirror_parent / zip_file.name)
        work = directory / "workspace"
        work.mkdir()
        (work / "main.tf").write_text(
            'terraform {\n'
            ' required_providers {\n'
            '  tplink = {\n'
            f'   source = "{SOURCE}"\n'
            f'   version = "{version}"\n'
            '  }\n'
            ' }\n'
            '}\n',
            encoding="utf-8",
        )
        (directory / "terraform.tfrc").write_text(
            'provider_installation {\n'
            ' filesystem_mirror {\n'
            f'  path = "{mirror.as_posix()}"\n'
            f'  include = ["{SOURCE}"]\n'
            ' }\n'
            '}\n',
            encoding="utf-8",
        )
        env = os.environ.copy()
        env["TF_CLI_CONFIG_FILE"] = str(directory / "terraform.tfrc")
        # Explicit no-network setup. If the archive isn't usable, init must
        # fail rather than downloading another provider as fallback.
        for args in (
            ("init", "-backend=false", "-input=false", "-no-color"),
            ("providers", "schema", "-json"),
        ):
            result = subprocess.run(
                [terraform, *args], cwd=work, env=env,
                text=True, capture_output=True, timeout=120,
            )
            if result.returncode:
                raise RuntimeError(
                    f"Offline registry package check failed on {args}: "
                    f"{result.stderr[:4000]} {result.stdout[:1500]}"
                )
        print(f"Offline registry ZIP installed and exposed schema: {zip_file.name}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit("Usage: validate_registry_package.py <semver> <os_arch>")
    main(sys.argv[1], sys.argv[2])

"""Build registry-compatible assets from an already frozen provider executable.

This command never signs, tags, or publishes anything.
"""
import json
import re
import sys
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PLATFORMS = {"linux_amd64", "darwin_arm64", "windows_amd64"}
VERSION = re.compile(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?$")


def package(version: str, platform: str, root: Path = ROOT) -> list[Path]:
    """Return zip and versioned manifest paths; reject malformed inputs early."""
    if not VERSION.fullmatch(version):
        raise ValueError(f"Expected bare SemVer such as 0.1.0, got {version!r}")
    if platform not in PLATFORMS:
        raise ValueError(f"Unsupported target platform {platform!r}")
    python_dir = root / "python"
    binary = root / "dist" / "python" / (
        "terraform-provider-tplink.exe" if platform.startswith("windows_")
        else "terraform-provider-tplink"
    )
    if not binary.is_file():
        raise FileNotFoundError(binary)
    src_manifest = python_dir / "terraform-registry-manifest.json"
    manifest = json.loads(src_manifest.read_text(encoding="utf-8"))
    if manifest != {"version": 1, "metadata": {"protocol_versions": ["6.0"]}}:
        raise ValueError("Expected Terraform protocol v6 registry manifest")
    license_file = python_dir / "LICENSE"
    notices = python_dir / "NOTICE.md"
    if not license_file.is_file() or not notices.is_file():
        raise FileNotFoundError("GPL license and source notices must be packaged")

    destination = root / "dist" / "release"
    destination.mkdir(parents=True, exist_ok=True)
    prefix = f"terraform-provider-tplink_{version}"
    manifest_path = destination / f"{prefix}_manifest.json"
    manifest_path.write_text(
        json.dumps(manifest, indent=2) + "\n", encoding="utf-8"
    )

    archive_path = destination / f"{prefix}_{platform}.zip"
    binary_name = f"terraform-provider-tplink_v{version}"
    if platform.startswith("windows_"):
        binary_name += ".exe"
    with zipfile.ZipFile(archive_path, "w", zipfile.ZIP_DEFLATED) as archive:
        archive.write(binary, binary_name)
        archive.write(license_file, "LICENSE")
        archive.write(notices, "NOTICE.md")
    with zipfile.ZipFile(archive_path) as archive:
        if sorted(archive.namelist()) != sorted(
            [binary_name, "LICENSE", "NOTICE.md"]
        ):
            raise RuntimeError("Release archive missing required files")
    return [archive_path, manifest_path]


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("Usage: package_release.py <bare-semver> <os_arch>")
    for path in package(sys.argv[1], sys.argv[2]):
        print(path)


if __name__ == "__main__":
    main()

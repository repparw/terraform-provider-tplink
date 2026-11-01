"""Package a frozen provider build into registry-compatible release assets.

Usage: python package_release.py <VERSION> <platform-suffix>
Expects the frozen binary at dist/python/terraform-provider-tplink* and
produces dist/release/terraform-provider-tplink_<VERSION>_<platform>.zip
containing the registry-named binary, plus copies the registry manifest.
"""
import json
import shutil
import zipfile
from pathlib import Path

root = Path(__file__).resolve().parent.parent


def main(version: str, platform: str) -> None:
    src = next((root / "dist" / "python").glob("terraform-provider-tplink*"))
    release = root / "dist" / "release"
    release.mkdir(parents=True, exist_ok=True)

    # Registry protocol: the binary inside the archive must be named
    # terraform-provider-tplink_v<VERSION>.
    binary_name = f"terraform-provider-tplink_v{version}"
    if platform.startswith("windows"):
        binary_name += ".exe"

    manifest_src = Path(__file__).parent / "terraform-registry-manifest.json"
    manifest_dst = release / "terraform-registry-manifest.json"
    if manifest_src.exists():
        shutil.copy(manifest_src, manifest_dst)
    else:
        manifest_dst.write_text(json.dumps(
            {"version": 1, "metadata": {"protocol_versions": ["6.0"]}}, indent=2))

    zip_path = release / f"terraform-provider-tplink_{version}_{platform}.zip"
    with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as zf:
        zf.write(src, binary_name)
        zf.write(manifest_src if manifest_src.exists() else manifest_dst,
                 "terraform-registry-manifest.json")
        for extra in ("LICENSE", "NOTICE.md"):
            p = Path(__file__).parent / extra
            if p.exists():
                zf.write(p, extra)
    print(f"wrote {zip_path}")


if __name__ == "__main__":
    import sys
    main(sys.argv[1], sys.argv[2])

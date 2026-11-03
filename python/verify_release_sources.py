"""Audit a per-platform release source archive before it can be published.

This checks the archive inventory and integrity without router access or
credentials. It complements, but does not replace, human license review.
"""
import hashlib
import json
from pathlib import Path, PurePosixPath
import sys
import tarfile
import zipfile
from io import BytesIO

from package_release import PLATFORMS, ROOT, VERSION


def verify(version: str, platform: str, root: Path = ROOT) -> None:
    if not VERSION.fullmatch(version) or platform not in PLATFORMS:
        raise ValueError("Invalid release version or platform")

    archive_path = root / "dist" / "release" / (
        f"tplink-provider-sources_{version}_{platform}.zip"
    )
    lock = json.loads((root / "python" / "dependency-lock.json").read_text())
    with zipfile.ZipFile(archive_path) as archive:
        names = archive.namelist()
        if len(names) != len(set(names)):
            raise RuntimeError("Duplicate entries in corresponding-source archive")
        if any(
            PurePosixPath(name).is_absolute()
            or ".." in PurePosixPath(name).parts
            for name in names
        ):
            raise RuntimeError("Unsafe path in corresponding-source archive")
        if archive.testzip() is not None:
            raise RuntimeError("Corrupt corresponding-source archive")
        present = set(names)
        required = {
            "LICENSE",
            "NOTICE.md",
            "DEPENDENCIES.json",
            "sources/provider-repository.tar.gz",
            "sources/tplinkrouterc6u-pinned.tar.gz",
        }
        if not required <= present:
            raise RuntimeError(f"Missing release sources: {sorted(required - present)}")

        inventory = json.loads(archive.read("DEPENDENCIES.json"))
        if (inventory.get("version") != version
            or inventory.get("platform") != platform
            or inventory.get("router_library_commit") != lock["commit"]):
            raise RuntimeError("Corresponding-source inventory metadata mismatch")

        rows = inventory.get("distributions")
        if not isinstance(rows, list) or not rows:
            raise RuntimeError("Empty corresponding-source dependency inventory")
        seen = set()
        for row in rows:
            name = row["name"].lower().replace("_", "-")
            if name in seen:
                raise RuntimeError(f"Duplicate dependency: {name}")
            seen.add(name)

            source = row["source"]
            if not isinstance(source, str) or not source.startswith("sources/") or source not in present:
                raise RuntimeError(f"Missing source distribution for {name}")
            raw = archive.read(source)
            if not raw:
                raise RuntimeError(f"Empty source distribution for {name}")
            if row.get("source_sha256") and hashlib.sha256(raw).hexdigest() != row["source_sha256"]:
                raise RuntimeError(f"Source checksum mismatch for {name}")

            license_paths = row["license_files"]
            if not isinstance(license_paths, list):
                raise RuntimeError(f"Invalid license file inventory for {name}")
            for filename in license_paths:
                if filename not in present or not archive.read(filename):
                    raise RuntimeError(f"Missing/empty dependency license: {filename}")
            license_label = str(row.get("license", "")).strip()
            if not license_paths and license_label.upper() in {"", "UNKNOWN", "NONE"}:
                raise RuntimeError(f"No usable license evidence for {name}")

        if not {"terraform-provider-tplink-python", "tplinkrouterc6u", "pyvider"} <= seen:
            raise RuntimeError("Provider or required router/library source is absent")

        with tarfile.open(fileobj=BytesIO(archive.read("sources/provider-repository.tar.gz")), mode="r:gz") as source:
            filenames = set(source.getnames())
            if not {"python/tplink_py/__init__.py", "python/LICENSE", "python/NOTICE.md"} <= filenames:
                raise RuntimeError("Provider source archive is incomplete")
        with tarfile.open(fileobj=BytesIO(archive.read("sources/tplinkrouterc6u-pinned.tar.gz")), mode="r:gz") as source:
            if "tplinkrouterc6u/client/sg.py" not in source.getnames():
                raise RuntimeError("Pinned SG router library source is missing")

    print(f"Verified release sources and licenses inventory: {archive_path.name}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit("Usage: verify_release_sources.py <version> <platform>")
    verify(sys.argv[1], sys.argv[2])

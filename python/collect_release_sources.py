"""Collect corresponding sources and license metadata for a frozen release.

This is intentionally a release gate. Missing source distributions, missing
license material or a moved router-library pin must fail publication.
Run on each target OS because the frozen dependency set may differ by OS.
"""
import hashlib
from importlib import metadata
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import zipfile

from packaging.requirements import Requirement
from packaging.utils import canonicalize_name

from package_release import ROOT, PLATFORMS, VERSION


def command(*args, cwd=None):
    result = subprocess.run(
        [str(x) for x in args], cwd=cwd, capture_output=True,
        check=True, timeout=300,
    )
    return result.stdout


def dependencies():
    """Resolve installed distributions reachable from the frozen entrypoint."""
    pending = ["terraform-provider-tplink-python", "pyinstaller"]
    found = {}
    while pending:
        name = canonicalize_name(pending.pop())
        if name in found:
            continue
        dist = metadata.distribution(name)
        found[name] = dist
        for requirement in dist.requires or ():
            req = Requirement(requirement)
            # Optional extras are not part of the installed runtime graph.
            if req.marker and not req.marker.evaluate({"extra": ""}):
                continue
            child = canonicalize_name(req.name)
            if child not in found:
                pending.append(child)
    if "tplinkrouterc6u" not in found or "pyvider" not in found:
        raise RuntimeError("Provider dependencies missing from installed graph")
    return found


def license_files(distribution):
    """Include packaged license texts, not only metadata classifications."""
    for entry in distribution.files or ():
        name = str(entry).replace("\\", "/")
        if re.search(r"(^|/)(LICENSE[^/]*|LICENCE[^/]*|COPYING[^/]*|NOTICE[^/]*)$", name, re.I):
            path = Path(distribution.locate_file(entry))
            if path.is_file():
                yield name, path.read_bytes()


def collect(version, platform, root=ROOT):
    if not VERSION.fullmatch(version) or platform not in PLATFORMS:
        raise ValueError("Invalid release version or platform")
    lock = json.loads((root / "python" / "dependency-lock.json").read_text())
    found = dependencies()
    result_dir = root / "dist" / "release"
    result_dir.mkdir(parents=True, exist_ok=True)
    output = result_dir / f"terraform-provider-tplink_{version}_{platform}_source.zip"

    with tempfile.TemporaryDirectory(prefix="tplink-sources-") as work:
        temp = Path(work)
        fork = temp / "router-library"
        command("git", "clone", "--quiet", "--depth", "1",
                "--branch", lock["tag"],
                f"https://github.com/{lock['repository']}.git", fork)
        actual = command("git", "rev-parse", "HEAD", cwd=fork).decode().strip()
        if actual != lock["commit"]:
            raise RuntimeError("Pinned router source differs from reviewed commit")
        own_source = command("git", "archive", "--format=tar.gz",
                             "HEAD", "python", cwd=root)
        fork_source = command("git", "archive", "--format=tar.gz",
                              "HEAD", cwd=fork)

        inventory = []
        with zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as archive:
            archive.writestr("sources/provider-repository.tar.gz", own_source)
            archive.writestr("sources/tplinkrouterc6u-pinned.tar.gz", fork_source)
            archive.write(root / "python" / "LICENSE", "LICENSE")
            archive.write(root / "python" / "NOTICE.md", "NOTICE.md")
            for name, dist in sorted(found.items()):
                version_installed = dist.version
                row = {
                    "name": dist.metadata.get("Name", name),
                    "version": version_installed,
                    "license": dist.metadata.get("License-Expression")
                        or dist.metadata.get("License") or "",
                    "home_page": dist.metadata.get("Home-page") or "",
                    "source": None,
                    "license_files": [],
                }
                for path, data in license_files(dist):
                    target = f"licenses/{name}/{path}"
                    archive.writestr(target, data)
                    row["license_files"].append(target)
                if name == "terraform-provider-tplink-python":
                    row["source"] = "sources/provider-repository.tar.gz"
                elif name == "tplinkrouterc6u":
                    row["source"] = "sources/tplinkrouterc6u-pinned.tar.gz"
                else:
                    source_dir = temp / name
                    source_dir.mkdir()
                    command(sys.executable, "-m", "pip", "download",
                            "--no-deps", "--no-binary=:all:",
                            "--dest", source_dir,
                            f"{row['name']}=={version_installed}")
                    artifacts = [p for p in source_dir.iterdir() if p.is_file()]
                    if len(artifacts) != 1 or not artifacts[0].name.endswith(
                            (".tar.gz", ".tar.bz2", ".tar.xz", ".zip")):
                        raise RuntimeError(f"Missing source distribution: {name}")
                    source = artifacts[0]
                    row["source"] = f"sources/{source.name}"
                    archive.write(source, row["source"])
                if not row["license_files"] and not row["license"]:
                    raise RuntimeError(f"No license evidence for {name}")
                inventory.append(row)
            archive.writestr(
                "DEPENDENCIES.json",
                json.dumps({"version": version, "platform": platform,
                            "router_library_commit": lock["commit"],
                            "distributions": inventory}, indent=2) + "\n",
            )
    print(f"Collected source and license archive: {output}")
    return output


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit("Usage: collect_release_sources.py <version> <platform>")
    collect(sys.argv[1], sys.argv[2])

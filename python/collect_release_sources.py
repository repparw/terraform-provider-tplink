"""Collect corresponding sources and license metadata for a frozen release.

This is intentionally a release gate. Missing source distributions, missing
license material or a moved router-library pin must fail publication.
Run on each target OS because the frozen dependency set may differ by OS.
"""
from importlib import metadata
from io import BytesIO
import json
import tarfile
from urllib.request import urlopen
from pathlib import Path
import re
import hashlib
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


def pypi_source(name, version):
    """Fetch source distribution bytes and check the PyPI-published SHA-256."""
    url = f"https://pypi.org/pypi/{name}/{version}/json"
    with urlopen(url, timeout=30) as response:
        release = json.load(response)
    candidates = [item for item in release["urls"]
                  if item.get("packagetype") == "sdist"]
    if len(candidates) != 1:
        raise RuntimeError(f"Expected exactly one sdist for {name}=={version}")
    artifact = candidates[0]
    with urlopen(artifact["url"], timeout=120) as response:
        data = response.read()
    if hashlib.sha256(data).hexdigest() != artifact["digests"]["sha256"]:
        raise RuntimeError(f"Source distribution hash mismatch: {name}")
    return artifact["filename"], data


def google_legacy_notice(version, source):
    """Recover google 3.0.0's BSD notice from its source setup.py.

    This package has License: UNKNOWN and no installed LICENSE file, but its
    PyPI source distribution explicitly carries a BSD-3-Clause header.
    Reject unexpected versions or changed headers rather than inventing a
    license from a PyPI classifier.
    """
    if version != "3.0.0":
        raise RuntimeError("Review changed google dependency license before release")
    with tarfile.open(fileobj=BytesIO(source), mode="r:gz") as archive:
        entry = archive.extractfile("google-3.0.0/setup.py")
        if entry is None:
            raise RuntimeError("google source missing setup.py license header")
        setup = entry.read().decode("utf-8")
    header, separator, _ = setup.partition("\nfrom os import chdir")
    required = (
        "Copyright (c) 2009-2020, Mario Vilas",
        "Redistribution and use in source and binary forms",
        "Neither the name of the copyright holder",
        "THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS",
    )
    if not separator or any(clause not in header for clause in required):
        raise RuntimeError("google source BSD license text changed; review required")
    return (header.strip() + "\n").encode("utf-8")


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
    output = result_dir / f"tplink-provider-sources_{version}_{platform}.zip"

    with tempfile.TemporaryDirectory(prefix="tplink-sources-") as work:
        temp = Path(work)
        fork = temp / "router-library"
        command("git", "clone", "--quiet", "--depth", "1",
                "--branch", lock["ref"].removeprefix("refs/heads/"),
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
                    filename, data = pypi_source(row["name"], version_installed)
                    if not filename.endswith((".tar.gz", ".tar.bz2", ".tar.xz", ".zip")):
                        raise RuntimeError(f"Unexpected source format for {name}")
                    row["source"] = f"sources/{filename}"
                    row["source_sha256"] = hashlib.sha256(data).hexdigest()
                    archive.writestr(row["source"], data)
                    if name == "google":
                        # The 3.0.0 distribution's metadata says UNKNOWN and
                        # it installs no standalone notice, but setup.py
                        # contains the full BSD-3-Clause redistribution terms.
                        notice = google_legacy_notice(version_installed, data)
                        target = "licenses/google/BSD-3-Clause-from-setup.py.txt"
                        archive.writestr(target, notice)
                        row["license_files"].append(target)
                        row["license"] = "BSD-3-Clause (source setup.py)"
                if not row["license_files"] and str(row["license"]).strip().upper() in {
                    "", "UNKNOWN", "NONE"
                }:
                    raise RuntimeError(f"No usable license evidence for {name}")
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

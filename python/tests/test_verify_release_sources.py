"""Offline regression tests for the corresponding-source release gate."""
from io import BytesIO
import hashlib
import json
from pathlib import Path
import sys
import tarfile
import zipfile

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from verify_release_sources import verify  # noqa: E402


def tarball(files):
    buffer = BytesIO()
    with tarfile.open(fileobj=buffer, mode="w:gz") as tar:
        for name, data in files.items():
            info = tarfile.TarInfo(name)
            info.size = len(data)
            tar.addfile(info, BytesIO(data))
    return buffer.getvalue()


@pytest.fixture()
def release(tmp_path):
    (tmp_path / "python").mkdir()
    directory = tmp_path / "dist" / "release"
    directory.mkdir(parents=True)
    (tmp_path / "python" / "dependency-lock.json").write_text(
        json.dumps({"commit": "a" * 40})
    )
    provider = tarball({
        "python/tplink_py/__init__.py": b"",
        "python/LICENSE": b"GPL",
        "python/NOTICE.md": b"notices",
    })
    library = tarball({"tplinkrouterc6u/client/sg.py": b"SG"})
    pyvider = b"pyvider-source"
    rows = [
        {"name": "terraform-provider-tplink-python", "source": "sources/provider-repository.tar.gz",
         "license": "GPL-3.0-or-later", "license_files": []},
        {"name": "tplinkrouterc6u", "source": "sources/tplinkrouterc6u-pinned.tar.gz",
         "license": "GPL-3.0-or-later", "license_files": []},
        {"name": "pyvider", "source": "sources/pyvider.tar.gz",
         "source_sha256": hashlib.sha256(pyvider).hexdigest(),
         "license": "", "license_files": ["licenses/pyvider/LICENSE"]},
    ]
    entries = {
        "LICENSE": b"GPL",
        "NOTICE.md": b"notices",
        "sources/provider-repository.tar.gz": provider,
        "sources/tplinkrouterc6u-pinned.tar.gz": library,
        "sources/pyvider.tar.gz": pyvider,
        "licenses/pyvider/LICENSE": b"Apache-2.0",
    }
    path = directory / "tplink-provider-sources_0.1.0_linux_amd64.zip"

    def write(*, rows_override=None, entries_override=None):
        current = entries | (entries_override or {})
        current["DEPENDENCIES.json"] = json.dumps({
            "version": "0.1.0", "platform": "linux_amd64",
            "router_library_commit": "a" * 40,
            "distributions": rows if rows_override is None else rows_override,
        }).encode()
        with zipfile.ZipFile(path, "w") as archive:
            for name, data in current.items():
                archive.writestr(name, data)
    write()
    return tmp_path, rows, write


def test_valid_corresponding_sources(release):
    root, _, _ = release
    verify("0.1.0", "linux_amd64", root)


def test_reject_changed_dependency_source(release):
    root, _, write = release
    write(entries_override={"sources/pyvider.tar.gz": b"tampered"})
    with pytest.raises(RuntimeError, match="checksum mismatch"):
        verify("0.1.0", "linux_amd64", root)


def test_reject_missing_license_evidence(release):
    root, rows, write = release
    modified = [dict(row) for row in rows]
    modified[-1]["license_files"] = []
    write(rows_override=modified)
    with pytest.raises(RuntimeError, match="license evidence"):
        verify("0.1.0", "linux_amd64", root)


def test_reject_wrong_platform_or_commit(release):
    root, _, _ = release
    with pytest.raises(FileNotFoundError):
        verify("0.1.0", "windows_amd64", root)
    lock = root / "python" / "dependency-lock.json"
    lock.write_text(json.dumps({"commit": "b" * 40}))
    with pytest.raises(RuntimeError, match="metadata mismatch"):
        verify("0.1.0", "linux_amd64", root)

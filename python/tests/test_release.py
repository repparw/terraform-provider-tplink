"""Release-archive contracts. Runs without router, Terraform or credentials."""
import json
import sys
import zipfile
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from package_release import package  # noqa: E402


@pytest.fixture()
def root(tmp_path):
    (tmp_path / "python").mkdir()
    (tmp_path / "dist" / "python").mkdir(parents=True)
    (tmp_path / "python" / "LICENSE").write_text("GPL test")
    (tmp_path / "python" / "NOTICE.md").write_text("notice")
    (tmp_path / "python" / "terraform-registry-manifest.json").write_text(
        json.dumps({"version": 1, "metadata": {"protocol_versions": ["6.0"]}})
    )
    (tmp_path / "dist" / "python" / "terraform-provider-tplink").write_bytes(b"ELF")
    (tmp_path / "dist" / "python" / "terraform-provider-tplink.exe").write_bytes(b"EXE")
    return tmp_path


@pytest.mark.parametrize("platform,suffix", [
    ("linux_amd64", ""), ("darwin_arm64", ""), ("windows_amd64", ".exe")
])
def test_registry_asset_names_and_manifest(root, platform, suffix):
    archive, manifest = package("0.1.0", platform, root)
    assert archive.name == f"terraform-provider-tplink_0.1.0_{platform}.zip"
    assert manifest.name == "terraform-provider-tplink_0.1.0_manifest.json"
    assert json.loads(manifest.read_text())["metadata"]["protocol_versions"] == ["6.0"]
    with zipfile.ZipFile(archive) as z:
        assert set(z.namelist()) == {
            f"terraform-provider-tplink_v0.1.0{suffix}", "LICENSE", "NOTICE.md"
        }


@pytest.mark.parametrize("version", [
    "pyprovider-v0.1.0", "v0.1.0", "0.1", "../0.1.0", "01.2.3", "1.2.3+bad"
])
def test_reject_invalid_versions_before_writing(root, version):
    with pytest.raises(ValueError):
        package(version, "linux_amd64", root)
    assert not (root / "dist" / "release").exists()


@pytest.mark.parametrize("platform", [
    "linux-amd64", "darwin/arm64", "windows_386"
])
def test_reject_invalid_platforms(root, platform):
    with pytest.raises(ValueError):
        package("0.1.0", platform, root)


def test_reject_incorrect_manifest_and_missing_notices(root):
    source = root / "python" / "terraform-registry-manifest.json"
    source.write_text('{"version": 1, "metadata": {"protocol_versions": ["5.0"]}}')
    with pytest.raises(ValueError):
        package("0.1.0", "linux_amd64", root)
    source.write_text('{"version": 1, "metadata": {"protocol_versions": ["6.0"]}}')
    (root / "python" / "LICENSE").unlink()
    with pytest.raises(FileNotFoundError):
        package("0.1.0", "linux_amd64", root)

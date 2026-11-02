"""Offline lock contract; the CI pre-install check verifies the actual remote."""
import sys
from pathlib import Path
import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from check_dependency_pin import check  # noqa: E402


def test_expected_sha_matches_python_dependency():
    lock = check(Path(__file__).resolve().parents[2], remote=False)
    assert lock["commit"] == "588f74f57a35e3818deb40825c8788d0240a2f62"


def test_dependency_ref_mismatch_fails(tmp_path):
    python = tmp_path / "python"
    python.mkdir()
    (python / "dependency-lock.json").write_text(
        '{"repository":"repparw/TP-Link-Archer-C6U","ref":"refs/heads/moved","commit":"588f74f57a35e3818deb40825c8788d0240a2f62"}'
    )
    (python / "pyproject.toml").write_text(
        '[project]\ndependencies = ["tplinkrouterc6u @ git+https://github.com/repparw/TP-Link-Archer-C6U.git@08a77c8e55142d91b9affef4ac91097a4a873bbd"]'
    )
    with pytest.raises(ValueError, match="differs"):
        check(tmp_path, remote=False)

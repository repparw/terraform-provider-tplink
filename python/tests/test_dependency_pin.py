"""Offline lock contract; the CI pre-install check verifies the actual remote."""
import sys
from pathlib import Path
import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from check_dependency_pin import check  # noqa: E402


def test_expected_tag_matches_python_dependency():
    lock = check(Path(__file__).resolve().parents[2], remote=False)
    assert lock["commit"] == "b40ab375b51da06b1d87bb318513c1e262770fdc"


def test_dependency_ref_mismatch_fails(tmp_path):
    python = tmp_path / "python"
    python.mkdir()
    (python / "dependency-lock.json").write_text(
        '{"repository":"repparw/TP-Link-Archer-C6U","tag":"moved","commit":"b40ab375b51da06b1d87bb318513c1e262770fdc"}'
    )
    (python / "pyproject.toml").write_text(
        '[project]\ndependencies = ["tplinkrouterc6u @ git+https://github.com/repparw/TP-Link-Archer-C6U.git@b40ab375b51da06b1d87bb318513c1e262770fdc"]'
    )
    with pytest.raises(ValueError, match="differs"):
        check(tmp_path, remote=False)

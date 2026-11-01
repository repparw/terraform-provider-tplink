"""Fail builds if the pinned TP-Link library tag drifts to another commit."""
import json
from pathlib import Path
import re
import subprocess
import sys
import tomllib


def check(root: Path, *, remote=True):
    lock = json.loads((root / "python" / "dependency-lock.json").read_text())
    dependencies = tomllib.loads(
        (root / "python" / "pyproject.toml").read_text()
    )["project"]["dependencies"]
    pinned = [dep for dep in dependencies if dep.startswith("tplinkrouterc6u @ ")]
    if len(pinned) != 1:
        raise ValueError("Expected exactly one Git-pinned tplinkrouterc6u dependency")
    url = f"https://github.com/{lock['repository']}.git"
    if pinned[0] != f"tplinkrouterc6u @ git+{url}@{lock['tag']}":
        raise ValueError("The declared dependency differs from dependency-lock.json")
    if not re.fullmatch("[a-f0-9]{40}", lock["commit"]):
        raise ValueError("Expected full, immutable 40-character commit SHA")
    if remote:
        refs = subprocess.run(
            ["git", "ls-remote", "--exit-code", "--refs", "--tags", url,
             f"refs/tags/{lock['tag']}"],
            text=True, capture_output=True, timeout=60, check=True,
        ).stdout.strip().splitlines()
        if len(refs) != 1 or refs[0].split()[0] != lock["commit"]:
            raise RuntimeError(
                "Pinned library tag moved, disappeared, or no longer matches "
                "the reviewed commit. Stop build and investigate."
            )
    return lock


if __name__ == "__main__":
    check(Path(__file__).resolve().parent.parent)
    print("Pinned library tag matches reviewed SHA")

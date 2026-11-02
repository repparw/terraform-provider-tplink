"""Read Terraform schema from the *bundled executable*, no router credentials."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile


def main():
    root = Path(__file__).resolve().parent.parent
    binary = root / "dist" / "python" / (
        "terraform-provider-tplink.exe" if os.name == "nt"
        else "terraform-provider-tplink"
    )
    assert binary.is_file(), binary
    expected_version = os.environ.get("PYVIDER_PROVIDER_VERSION", "0.0.0")
    version_env = os.environ.copy()
    # An installed provider must ignore an unrelated runtime environment
    # override. The frozen build's version comes from its runtime hook.
    version_env["PYVIDER_PROVIDER_VERSION"] = "99.99.99"
    reported = subprocess.run(
        [str(binary), "--tplink-build-version"],
        env=version_env, text=True, capture_output=True, timeout=60,
    )
    if reported.returncode or reported.stdout.strip() != expected_version:
        raise RuntimeError(
            "Frozen version mismatch: expected "
            f"{expected_version!r}, got {reported.stdout.strip()!r}; "
            f"stderr: {reported.stderr[:2000]}"
        )
    terraform = shutil.which("terraform")
    assert terraform, "Terraform must be installed by CI"
    with tempfile.TemporaryDirectory(prefix="tplink-schema-") as temp:
        directory = Path(temp)
        (directory / "main.tf").write_text(
            'terraform {\n'
            '  required_providers {\n'
            '    tplink = {\n'
            '      source = "local/providers/tplink"\n'
            '      version = "0.0.0"\n'
            '    }\n'
            '  }\n'
            '}\n'
        )
        (directory / "provider.tfrc").write_text(
            'provider_installation {\n'
            '  dev_overrides {\n'
            f'    "local/providers/tplink" = "{binary.parent.as_posix()}"\n'
            '  }\n'
            '  direct {}\n'
            '}\n'
        )
        env = os.environ.copy()
        env["TF_CLI_CONFIG_FILE"] = str(directory / "provider.tfrc")
        result = subprocess.run(
            [terraform, "providers", "schema", "-json"],
            env=env, cwd=directory,
            text=True, capture_output=True, timeout=90,
        )
        if result.returncode:
            raise RuntimeError(
                f"Frozen provider schema failed (exit {result.returncode}): "
                f"{result.stderr[:5000]}")
        schemas = json.loads(result.stdout)["provider_schemas"]
        assert len(schemas) == 1, schemas.keys()
        data = next(iter(schemas.values()))
        attrs = data["provider"]["block"]["attributes"]
        assert {"host", "password", "username", "tls_skip_verify", "timeout_seconds",
                "password_key_pin", "auth_key_pin"} <= attrs.keys()
        assert {"tplink_port_forward", "tplink_dhcp_reservation"} <= (
            data["resource_schemas"].keys()
        )
        assert {"tplink_router", "tplink_firmware", "tplink_devices",
                "tplink_dhcp_reservations", "tplink_port_forwards"} <= (
            data["data_source_schemas"].keys()
        )
        print("Frozen provider schema verified with Terraform protocol 6")


if __name__ == "__main__":
    main()

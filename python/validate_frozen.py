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
    terraform = shutil.which("terraform")
    assert terraform, "Terraform must be installed by CI"
    with tempfile.TemporaryDirectory(prefix="tplink-schema-") as temp:
        directory = Path(temp)
        (directory / "main.tf").write_text(
            'terraform {\n'
            '  required_providers {\n'
            '    tplink = { source = "local/providers/tplink" '
            'version = "0.0.0" }\n'
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
        assert {"host", "password", "username", "verify_ssl"} <= attrs.keys()
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

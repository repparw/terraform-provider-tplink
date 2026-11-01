# Provider release checklist

The project publishes Terraform/OpenTofu provider ZIP archives through GitHub
Releases. Registry registration is a **separate** process; no registry listing
or release is created merely by merging this tooling.

The released implementation is the **Python/Pyvider provider** (`python/`,
GPL-3.0-or-later). Tags follow `pyprovider-v<semver>` and trigger
`.github/workflows/python-release.yml`, which builds frozen executables on
Linux/macOS/Windows, packages registry-compatible archives, and signs the
checksums with GPG through the `publish` environment.

**NixOS prerequisite**: the frozen executables are PyInstaller builds that
resolve their ELF interpreter via `/lib64/ld-linux-x86-64.so.2`. NixOS hosts
need `programs.nix-ld.enable = true;` (the fleet alpha already sets this).
Without it, the binary fails with 'No such file or directory' before any
code runs. This is the only runtime requirement beyond OpenTofu/Terraform
itself; no Python installation is needed on the target machine.

## Prerequisites

1. Review the compatibility scope: only Archer AX23 v2.0 firmware
   1.1.2 Build 20250814 has been tested with live writes.
2. Key pinning is implemented and hardware-verified (PR #9 in the Go
   client; the Python provider exposes `password_key_pin` / `auth_key_pin`
   provider attributes backed by the pinned fork).
3. Keep `docs/` and `examples/` current with the provider schema (they are
   static; verify attributes against `tofu providers schema -json`).
4. Verify the PR CI checks, including the frozen-executable packaging and
   schema handshake jobs.
5. Create a dedicated GitHub Actions environment named `publish`,
   **require maintainer approval**, and store these **environment secrets**:
   - `GPG_PRIVATE_KEY`: ASCII-armored, exported private signing key.
   - `PASSPHRASE`: signing key passphrase.
   Never commit private key material, credentials, or passphrases.
6. Register the **public** signing key with the target registry.
   OpenTofu requires a separate signing-key submission through its issue form.
   Terraform Registry requires provider registration and verification.

## Cut a release

After review, update CHANGELOG.md with the release date, ensure the tag
refers to the intended merged commit, then push a signed or annotated
SemVer tag like `v0.1.0`. The tag triggers `.github/workflows/release.yml`.
Review and approve the `publish` environment job before any assets are
uploaded. The workflow imports the signing key, builds the platform bundles, and
publishes the release.

Expected GitHub Release assets:

- `terraform-provider-tplink_<VERSION>_<OS>_<ARCH>.zip` archives containing
  the frozen `terraform-provider-tplink_v<VERSION>` executable, the registry
  manifest, and the GPL license/notice files.
- `terraform-provider-tplink_<VERSION>_manifest.json` with protocol 6.0.
- `terraform-provider-tplink_<VERSION>_SHA256SUMS`, including the
  manifest and archives.
- `terraform-provider-tplink_<VERSION>_SHA256SUMS.sig`, a **binary detached
  GPG signature** (not an ASCII-armored signature).

Validate the checksum signature and test `tofu init` / `terraform init`
using registry-host-correct source addresses after registration. GitHub
release archives and Terraform Registry publication are not substitutes for
testing the complete installation flow.

## Registry and compatibility notes

The provider server currently advertises
`registry.opentofu.org/repparw/tplink`. Publishing to Terraform Registry
(`registry.terraform.io/repparw/tplink`) must include a separate source-address
and schema smoke test; do not assume the same build automatically works with
a different provider identity.

**Verified hardware**: only the Archer AX23 v2.0 (firmware 1.1.2 Build
20250814) has been tested with live reads and writes. v0.1.0 advertises
exactly that scope; models sharing the web UI are untested. The Python
provider's GPL-3.0-or-later license applies to the released archives
(the Go implementation remains MIT on main until retired).

The publish environment and secrets must be configured by a repository owner
before the first tag. Do not create tags while this is unconfigured.

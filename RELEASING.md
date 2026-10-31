# Provider release checklist

The project publishes Terraform/OpenTofu provider ZIP archives through GitHub
Releases. Registry registration is a **separate** process; no registry listing
or release is created merely by merging this tooling.

## Prerequisites

1. Review the compatibility scope: only Archer AX23 v2.0 firmware
   1.1.2 Build 20250814 has been tested with live writes.
2. Finish the key-pinning investigation and any selected follow-up fixes.
3. Regenerate docs with
   `go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.24.0 generate --provider-name=tplink`,
   using Terraform 1.17.1, then commit `docs/`.
4. Verify the PR CI checks, including the unsigned GoReleaser snapshot.
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
uploaded. The workflow imports the signing key and runs GoReleaser.

Expected GitHub Release assets:

- `terraform-provider-tplink_<VERSION>_<OS>_<ARCH>.zip` archives containing
  `terraform-provider-tplink_v<VERSION>` provider binaries.
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
a different provider identity. Documentation must distinguish verified hardware
from models that merely appear to share the same web UI.

The publish environment and secrets must be configured by a repository owner
before the first tag. Do not create tags while this is unconfigured.

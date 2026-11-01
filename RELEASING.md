# Python provider release checklist

The Python/Pyvider implementation is the intended v0.1.0 release (GPL-3.0-or-later).
**Do not tag, merge, or publish while PR #8 is draft.** A GitHub release is
separate from registry registration, and publishing is restricted by the
required-reviewer `publish` environment.

## Verified scope and limitations

- Hardware writes verified only on Archer AX23 v2.0, firmware 1.1.2 Build 20250814.
- Frozen Linux binary requires a compatible glibc loader. Stock NixOS needs
  `programs.nix-ld.enable = true;` and any required dynamic libraries such
  as `libz` available to nix-ld. A standalone NixOS packaging option is open.
- The SG RSA pinning implementation exists in
  [the upstream-library fork PR #3](https://github.com/repparw/TP-Link-Archer-C6U/pull/3),
  but the current provider dependency tag predates it. The provider refuses
  configured pins rather than silently disabling them. Pin support must be
  repinned, reviewed and tested before claiming pinning works on Python.
- Passive-first auto-detection is not in the Python release. AX23 uses SG
  explicitly and avoids legacy credential-based probing.

## Before the first release

1. Stabilize the upstream Python-library dependency and keep its source
   available. Prefer an immutable reviewed reference; verify the tag's SHA.
2. Validate both optional RSA pins against isolated tests and, with explicit
   hardware authorization, the real router. Do not hardcode fingerprint values.
3. Update `docs/` and `examples/` to match the actual provider schema.
4. Review license compatibility and create a complete transitive dependency
   attribution/source bundle. The frozen Python provider is GPL-covered; the
   previous Go implementation remains MIT.
5. Review CI on PR #8: unit tests, Terraform simulated CRUD/import/replace,
   and Linux/macOS/Windows frozen-binary schema and archive-format checks.
6. Test published-package installation with Terraform and OpenTofu using
   correct registry addresses; only the development provider has been
   hardware-validated on AX23.
7. Configure the GitHub Actions `publish` environment with mandatory maintainer
   approval and restricted trusted tag-creation permissions. Store
   `GPG_PRIVATE_KEY` and `PASSPHRASE` as environment secrets. Register the
   public RSA/DSA GPG signing key with Terraform Registry and separately
   with the OpenTofu Registry. Never commit a private key.

## Release procedure

Only after the PR is merged and its checks pass, create an annotated or
signed tag `pyprovider-v0.1.0` pointing at the intended release commit.
The `.github/workflows/python-release.yml` workflow builds three
platforms, runs a real Terraform schema handshake and packages zip files.
It does **not** publish until the `publish` environment job is approved.
That job validates the artifacts, signs their checksums, verifies the
signature and creates the GitHub Release. A user must then complete the
registry registration separately.

Expected assets for 0.1.0:

- `terraform-provider-tplink_0.1.0_linux_amd64.zip`
- `terraform-provider-tplink_0.1.0_darwin_arm64.zip`
- `terraform-provider-tplink_0.1.0_windows_amd64.zip`
- `terraform-provider-tplink_0.1.0_manifest.json` (protocol 6.0)
- `terraform-provider-tplink_0.1.0_SHA256SUMS`
- `terraform-provider-tplink_0.1.0_SHA256SUMS.sig` (binary detached GPG)

Each archive contains the frozen `terraform-provider-tplink_v0.1.0`
executable (with `.exe` on Windows), the GPL license and notices.
The checksum file covers the three ZIPs and the versioned manifest.

Verify the signature against the published public signing key and the zip
checksums before registering the release. Confirm independent
`terraform init` / `tofu init` using supported registry addresses and a
clean provider cache. Do not assume a release visible on GitHub is usable
from either registry.

## Documentation and scope

The Python provider's primary development identity is `tplink`. The
registry source addresses `registry.terraform.io/repparw/tplink` and
`registry.opentofu.org/repparw/tplink` are independently registered.
Neither address has been validated as published yet. Only AX23 v2.0
firmware 1.1.2 is supported until additional hardware is tested.

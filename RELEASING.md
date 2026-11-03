# Python provider release checklist

The Python/Pyvider implementation is the sole v0.1.0 release candidate (GPL-3.0-or-later). The Go provider and Go CI were retired before the first public release.
Provider PR #8 merged on 2026-11-02. Do not tag or publish until the source
archive audit, Alpha frozen-binary verification, and protected signing setup
are complete. A GitHub release is separate from registry registration, and
publication is restricted by the required-reviewer `publish` environment.

## Verified scope and limitations

- Hardware writes verified only on Archer AX23 v2.0, firmware 1.1.2 Build 20250814.
- Frozen Linux binary requires a compatible glibc loader. Stock NixOS needs
  `programs.nix-ld.enable = true;` and any required dynamic libraries such
  as `libz` available to nix-ld. A standalone NixOS packaging option is open.
- The pinned library commit `588f74f57a35e3818deb40825c8788d0240a2f62` includes both
  SG RSA public-key pin checks, and CI tests that specific commit. Correct pins
  and wrong password/authentication pins were hardware-validated on AX23 v2.
  A missing pin API or mismatched key fails closed.
- The provider only retries explicit router-reported session expiry. Unknown
  network timeouts are not retried, as writes may already have succeeded.
- Passive-first auto-detection is not in the Python release. AX23 uses SG
  explicitly and avoids legacy credential-based probing.

## Before the first release

1. Run `python python/check_dependency_pin.py` to confirm the pinned
   Python-library commit still resolves to the reviewed SHA in
   `python/dependency-lock.json`. Keep the matching source available.
2. Preserve the successful hardware pinning test evidence. Recheck pinning
   after any router firmware, dependency or key change with
   `python -m tplink_py.keyprobe` over a trusted connection. Compare the
   fingerprints independently before enrolling pins; never hardcode values.
3. Keep `docs/` and `examples/` in sync with the Python provider schema and the new Python inventory generator.
4. Review the CI-produced, per-platform `tplink-provider-sources_*.zip` archives.
   The automated `python/verify_release_sources.py` gate checks archive integrity,
   provider and router-library source presence, dependency source hashes, and
   the license inventory. This is not a substitute for a human review of the
   actual frozen binary's transitive components, corresponding-source coverage,
   and third-party license obligations. Historical Go versions retain MIT terms.
5. Review the latest release-candidate CI: unit tests, Terraform simulated
   CRUD/import/replace, and Linux/macOS/Windows frozen-binary schema, archive,
   and offline package-installation checks. Download the artifact from the
   *specific successful head commit* being considered for the release.
6. Before tagging, run the Linux release-candidate executable on Alpha and
   perform a read-only plan against the existing router/OpenTofu state.
   Do not modify household entries or use `tofu apply` for this release check.
   After registry publication, independently test fresh `terraform init` and
   `tofu init` with the real registry source addresses and clean caches.
7. Configure the GitHub Actions `publish` environment with mandatory maintainer
   approval and restricted trusted tag-creation permissions. Store
   `GPG_PRIVATE_KEY` and `PASSPHRASE` as environment secrets. Use an RSA/DSA
   GPG signing key (not an ECC-only key) for Terraform Registry compatibility.
   Register its public key with Terraform Registry and separately with OpenTofu.
   Never commit a private key.

## Release procedure

Only after the PR is merged and its checks pass, create an annotated or
signed tag `v0.1.0` pointing at the intended release commit. The public
Terraform Registry expects `v`-prefixed semantic-version tags; the old
`pyprovider-v*` convention cannot publish a registry version.
The `.github/workflows/python-release.yml` workflow builds three
platforms, runs a real Terraform schema handshake and packages binary ZIPs
plus matching source/license ZIPs.
It does **not** publish until the `publish` environment job is approved.
That job validates the artifacts, signs their checksums, verifies the
signature and creates the GitHub Release. A maintainer must then complete registry registration separately.

For Terraform Registry, sign in through GitHub, add the ASCII-armored public
GPG key in User Settings > Signing Keys, then use Publish > Provider to register
`repparw/terraform-provider-tplink` (see
[HashiCorp's publishing guide](https://developer.hashicorp.com/terraform/registry/providers/publishing)).

For OpenTofu Registry, use the **GitHub issue forms** in
[opentofu/registry](https://github.com/opentofu/registry):
[provider submission](https://github.com/opentofu/registry/issues/new?template=provider.yml)
and [provider signing-key submission](https://github.com/opentofu/registry/issues/new?template=provider_key.yml).
These must be submitted through the interactive issue-form UI. Its automation
explicitly rejects plain API/CLI-created issues and direct metadata PRs.
Allow for review before relying on `registry.opentofu.org/repparw/tplink`.

Expected assets for 0.1.0:

- `terraform-provider-tplink_0.1.0_linux_amd64.zip`
- `terraform-provider-tplink_0.1.0_darwin_arm64.zip`
- `terraform-provider-tplink_0.1.0_windows_amd64.zip`
- `tplink-provider-sources_0.1.0_linux_amd64.zip`
- `tplink-provider-sources_0.1.0_darwin_arm64.zip`
- `tplink-provider-sources_0.1.0_windows_amd64.zip`
- `terraform-provider-tplink_0.1.0_manifest.json` (protocol 6.0)
- `terraform-provider-tplink_0.1.0_SHA256SUMS`
- `terraform-provider-tplink_0.1.0_SHA256SUMS.sig` (binary detached GPG)

Each archive contains the frozen `terraform-provider-tplink_v0.1.0`
executable (with `.exe` on Windows), the GPL license and notices.
The checksum file covers all six ZIPs and the versioned manifest.

Verify the signature against the published public signing key and the zip
checksums before registering the release. Confirm independent
`terraform init` / `tofu init` using supported registry addresses and a
clean provider cache. Do not assume a release visible on GitHub is usable
from either registry.

## Documentation and scope

The provider's identity is `tplink`. The
registry source addresses `registry.terraform.io/repparw/tplink` and
`registry.opentofu.org/repparw/tplink` are independently registered.
Neither address has been validated as published yet. Only AX23 v2.0
firmware 1.1.2 is supported until additional hardware is tested.

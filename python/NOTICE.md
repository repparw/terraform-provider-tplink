# Python provider licensing

This directory contains the active Python provider implementation
under GPL-3.0-or-later. It depends on the GPL-3.0-or-later
`tplinkrouterc6u` router library, pinned by the immutable `588f74f57a35e3818deb40825c8788d0240a2f62` commit in `pyproject.toml` (SHA `588f74f57a35e3818deb40825c8788d0240a2f62`).

The retired Go implementation was distributed under MIT and remains available
under those original terms in Git history. The active provider and repository
are GPL-3.0-or-later. Historical MIT permissions are not revoked.

The corresponding Python provider source is under `python/` in this repository.
The pinned Python router library source is available at:
https://github.com/repparw/TP-Link-Archer-C6U/tree/588f74f57a35e3818deb40825c8788d0240a2f62

Pyvider itself is Apache-2.0. Other third-party dependencies retain their own licenses. The release process
builds per-platform corresponding-source ZIPs with dependency source archives,
license files and a dependency inventory. Publication requires those ZIPs;
review their completeness and compatibility before approving the release.

No public Python-provider release has been cut. Never distribute a frozen binary without its matching per-platform source and license archive.

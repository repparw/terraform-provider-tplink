# Python provider licensing

This directory contains the intended Python provider implementation
under GPL-3.0-or-later. It depends on the GPL-3.0-or-later
`tplinkrouterc6u` router library, pinned by the immutable `b40ab375b51da06b1d87bb318513c1e262770fdc` commit in `pyproject.toml` (SHA `b40ab375b51da06b1d87bb318513c1e262770fdc`).

The repository's existing Go implementation retains its own MIT license.
Do not advertise a frozen Python distribution as MIT-licensed.

The corresponding Python provider source is under `python/` in this repository.
The pinned Python router library source is available at:
https://github.com/repparw/TP-Link-Archer-C6U/tree/b40ab375b51da06b1d87bb318513c1e262770fdc

Pyvider itself is Apache-2.0. Other third-party dependencies retain their own licenses. The release process
builds per-platform corresponding-source ZIPs with dependency source archives,
license files and a dependency inventory. Publication requires those ZIPs;
review their completeness and compatibility before approving the release.

No public Python-provider release has been cut. Never distribute a frozen binary without its matching per-platform source and license archive.

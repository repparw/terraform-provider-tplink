# Python experiment licensing

This directory contains the intended Python provider implementation
under GPL-3.0-or-later. It depends on the GPL-3.0-or-later
`tplinkrouterc6u` router library, pinned by the `pyprovider-pin-20261031` tag in `pyproject.toml` (SHA `08a77c8e55142d91b9affef4ac91097a4a873bbd`).

The repository's existing Go implementation retains its own MIT license.
Do not advertise a frozen Python distribution as MIT-licensed.

The corresponding Python provider source is under `python/` in this repository.
The pinned Python router library source is available at:
https://github.com/repparw/TP-Link-Archer-C6U/tree/08a77c8e55142d91b9affef4ac91097a4a873bbd

Pyvider itself is Apache-2.0. Other third-party dependencies retain their
own licenses; prepare the complete transitive license bundle before any
registry or GitHub Release publication of the Python provider.

No public Python-provider release has been cut. Never distribute a frozen binary until its full transitive dependency licenses and corresponding source are packaged.

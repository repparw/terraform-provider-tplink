# Python experiment licensing

This directory contains a distinct experimental Python implementation
under GPL-3.0-or-later. It depends on the GPL-3.0-or-later
`tplinkrouterc6u` router library, pinned by commit in `pyproject.toml`.

The repository's existing Go implementation retains its own MIT license.
Do not advertise a frozen Python distribution as MIT-licensed.

The corresponding Python provider source is under `python/` in this repository.
The pinned Python router library source is available at:
https://github.com/repparw/TP-Link-Archer-C6U/tree/19cceb975ed3c1d3ce666e1b1dc2dda03899497e

Pyvider itself is Apache-2.0. Other third-party dependencies retain their
own licenses; prepare the complete transitive license bundle before any
registry or GitHub Release publication of the Python provider.

No public Python-provider release has been cut.

"""Offline inventory rendering tests. No router is contacted."""
import json
from pathlib import Path
from types import SimpleNamespace as Row

import pytest

from tplink_py.inventory import (
    canonical_mac, hcl_string, inventory_rows, render_imports,
    render_tfvars, write_private,
)


def test_canonical_mac_and_invalid_input():
    assert canonical_mac("aa:bb:cc:dd:ee:ff") == "AA-BB-CC-DD-EE-FF"
    assert canonical_mac("aabb.ccdd.eeff") == "AA-BB-CC-DD-EE-FF"
    with pytest.raises(ValueError):
        canonical_mac("AA-BB-CC-DD-EE-GG")


def test_template_escaping():
    quoted = hcl_string("rule-${device}-%{ if true }")
    assert quoted == '"rule-$${device}-%%{ if true }"'
    assert json.loads(hcl_string('test"\\')) == 'test"\\'


def test_inventory_rendering_and_import_keys():
    fw = Row(model="AX23", hardware_version="v2", firmware_version="1.1.2")
    reservations = [
        Row(macaddr="aa:bb:cc:dd:ee:ff", ipaddr="192.168.0.33",
            hostname="phone-${hostname}", enabled=True),
    ]
    forwards = [
        Row(name="rule-${literal}", ipaddr="192.168.0.4",
            external_port="443", internal_port="8443",
            protocol="TCP", enabled=False),
    ]
    dhcp, rules = inventory_rows(reservations, forwards)
    assert list(dhcp) == ["AA-BB-CC-DD-EE-FF"]
    vars_ = render_tfvars(fw, dhcp, rules)
    imports = render_imports(dhcp, rules)
    assert 'phone-$${hostname}' in vars_
    assert '"rule-$${literal}"' in imports
    assert "enabled = false" in vars_
    assert 'tplink_dhcp_reservation.entries["AA-BB-CC-DD-EE-FF"]' in imports


def test_duplicate_identities_rejected_before_write():
    row = Row(macaddr="aa:bb:cc:dd:ee:ff", ipaddr="192.168.0.33",
              hostname="x", enabled=True)
    with pytest.raises(ValueError, match="Duplicate reservation MAC"):
        inventory_rows([row, row], [])
    rule = Row(name="dup", ipaddr="192.168.0.4", external_port="80",
               internal_port="80", protocol="TCP", enabled=True)
    with pytest.raises(ValueError, match="duplicate"):
        inventory_rows([], [rule, rule])


def test_private_file_permissions(tmp_path: Path):
    output = tmp_path / "inventory.auto.tfvars"
    write_private(output, "test")
    assert output.read_text() == "test"
    assert output.stat().st_mode & 0o777 == 0o600

"""CI-only virtual router injection. Not included in the provider package.

Loaded only when PYTHONPATH explicitly points to tests/mock_site.
Stores state on disk so separate Terraform plugin processes see the same rows.
"""
import json
import os
from pathlib import Path
from types import SimpleNamespace

import tplinkrouterc6u

STORE = Path(os.environ["TPLINK_MOCK_STORE"]) if "TPLINK_MOCK_STORE" in os.environ else None


def read_db():
    return json.loads(STORE.read_text()) if STORE.exists() else {
        "forwards": [], "reservations": []
    }


def write_db(value):
    STORE.write_text(json.dumps(value, indent=2))


class FakeRouter:
    def __init__(self, host, password, username="admin", verify_ssl=True, **kwargs):
        self.host = host
        self.password = password
        self.username = username

    def authorize(self):
        return None

    def logout(self):
        return None

    def get_status(self):
        from tplinkrouterc6u.common.package_enum import Connection
        return SimpleNamespace(
            conn_type="dhcp", wan_ipv4_addr="192.0.2.10",
            wan_ipv4_gateway="192.0.2.1", wan_macaddr="AA-BB-CC-DD-EE-00",
            wan_ipv4_uptime=120, lan_ipv4_addr="192.168.0.1",
            lan_macaddr="AA-BB-CC-DD-EE-01", cpu_usage=0.12,
            mem_usage=0.23, clients_total=1, wired_total=1,
            wifi_clients_total=0, guest_clients_total=0,
            iot_clients_total=0,
            devices=[SimpleNamespace(
                hostname="printer", ipaddr="192.168.0.20",
                macaddr="AA-BB-CC-DD-EE-FF", type=Connection.WIRED)],
        )

    def get_ipv4_status(self):
        return SimpleNamespace(lan_ipv4_dhcp_enable=True)

    def get_firmware(self):
        return SimpleNamespace(model="CI-mock", hardware_version="1.0",
                               firmware_version="mock")

    def get_ipv4_port_forwards(self):
        return [SimpleNamespace(**row) for row in read_db()["forwards"]]

    def get_ipv4_reservations(self):
        return [SimpleNamespace(**row) for row in read_db()["reservations"]]

    def add_ipv4_port_forward(self, name, external_port, internal_port, ipaddr,
                              protocol="TCP", enable=True):
        db = read_db()
        assert all(row["name"] != name for row in db["forwards"])
        db["forwards"].append({
            "name": name, "external_port": external_port,
            "internal_port": internal_port, "ipaddr": ipaddr,
            "protocol": protocol.upper(), "enabled": enable,
        })
        write_db(db)

    def update_ipv4_port_forward(self, name, new_name=None, external_port=None,
                                 internal_port=None, ipaddr=None, protocol=None,
                                 enable=None):
        db = read_db()
        row = next(row for row in db["forwards"] if row["name"] == name)
        for key, value in (
            ("external_port", external_port), ("internal_port", internal_port),
            ("ipaddr", ipaddr), ("protocol", protocol.upper() if protocol else None),
            ("enabled", enable),
        ):
            if value is not None: row[key] = value
        if new_name is not None: row["name"] = new_name
        write_db(db)

    def delete_ipv4_port_forward(self, name):
        db = read_db()
        db["forwards"] = [row for row in db["forwards"] if row["name"] != name]
        write_db(db)

    def add_ipv4_reservation(self, macaddr, ipaddr, comment="", enable=True):
        from macaddress import EUI48
        db = read_db()
        mac = str(EUI48(macaddr))
        assert all(row["macaddr"] != mac for row in db["reservations"])
        db["reservations"].append({
            "macaddr": mac, "ipaddr": ipaddr, "hostname": comment,
            "enabled": enable,
        })
        write_db(db)

    def update_ipv4_reservation(self, macaddr, ipaddr=None, comment=None,
                                enable=None):
        from macaddress import EUI48
        db = read_db()
        mac = str(EUI48(macaddr))
        row = next(row for row in db["reservations"] if row["macaddr"] == mac)
        if ipaddr is not None: row["ipaddr"] = ipaddr
        if comment is not None: row["hostname"] = comment
        if enable is not None: row["enabled"] = enable
        write_db(db)

    def delete_ipv4_reservation(self, macaddr):
        from macaddress import EUI48
        db = read_db()
        mac = str(EUI48(macaddr))
        db["reservations"] = [
            row for row in db["reservations"] if row["macaddr"] != mac
        ]
        write_db(db)


tplinkrouterc6u.TplinkRouterSG = FakeRouter

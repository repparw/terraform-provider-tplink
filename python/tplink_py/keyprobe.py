"""Read SG public RSA key fingerprints without logging into the router.

The output is trustworthy only if obtained over a trusted connection.
It performs the two pre-authentication key reads, never a password login.
"""
import argparse
import os

from tplinkrouterc6u import TplinkRouterSG


def probe(host: str, timeout: int = 30, router_class=TplinkRouterSG):
    router = router_class(host, "", timeout=timeout)
    router._request_pwd_keys()
    router._request_auth_keys()
    return {
        "password_key_pin": router._fingerprint_spki(router._pwdNN, router._pwdEE),
        "auth_key_pin": router._fingerprint_spki(router._nn, router._ee),
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--host", default=os.getenv("TPLINK_HOST", "http://192.168.0.1"))
    parser.add_argument("--timeout", type=int, default=30)
    args = parser.parse_args(argv)
    if args.timeout <= 0:
        parser.error("--timeout must be positive")
    pins = probe(args.host, args.timeout)
    for key, value in pins.items():
        print(f"{key} = {value}")


if __name__ == "__main__":
    main()

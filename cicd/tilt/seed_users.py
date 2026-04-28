"""
Seed users for local development.

Creates each user (state=pending) then immediately activates them.
Skips users that already exist.

Usage:
    python seed_users.py
    python seed_users.py --gateway http://localhost:9001
"""
import argparse

import httpx

GATEWAY_DEFAULT = "http://localhost:9001"

SEED_USERS = [
    {"username": "root",   "email": "root@mercury.local",   "password": "password"},
    {"username": "tester", "email": "tester@mercury.local", "password": "password"},
    {"username": "user",   "email": "user@mercury.local",   "password": "password"},
    {"username": "bob",    "email": "bob@mercury.local",    "password": "password"},
    {"username": "alice",  "email": "alice@mercury.local",  "password": "password"},
]


def seed_user(client: httpx.Client, gateway: str, username: str, email: str, password: str) -> None:
    resp = client.post(f"{gateway}/api/v1/account",
                       json={"username": username, "email": email, "password": password})
    if resp.status_code != 200:
        print(f"  skipped  : {username} ({resp.status_code})")
        return
    account_id = resp.json()["account_id"]
    print(f"  created  : {username} ({email}) → {account_id}")

    resp = client.post(f"{gateway}/api/v1/account/activate/{account_id}")
    if resp.status_code != 200:
        print(f"  WARNING  : activation failed for {account_id} ({resp.status_code})")
        return
    print(f"  activated: {account_id}")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--gateway", "-g", default=GATEWAY_DEFAULT,
                        help=f"Gateway base URL (default: {GATEWAY_DEFAULT})")
    args = parser.parse_args()

    with httpx.Client(timeout=30.0) as client:
        for u in SEED_USERS:
            print(f"\nSeeding '{u['username']}' ...")
            seed_user(client, args.gateway, u["username"], u["email"], u["password"])

    print("\nDone.")


if __name__ == "__main__":
    main()

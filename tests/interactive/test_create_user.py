"""
Seed users via the auth service API.

Creates each user (state=pending) then immediately activates them.

Usage:
    python seed_users.py
    python seed_users.py --auth http://localhost:9005
"""
import argparse
import random
import string


import httpx

AUTH_DEFAULT = "http://localhost:9005"

SEED_USERS = [
    {"username": "root",   "email": "root@mercury.local"},
    {"username": "tester", "email": "tester@mercury.local"},
    {"username": "user", "email": "user@mercury.local"},
    {"username": "user2", "email": "user2@mercury.local"},
]


characters = string.ascii_letters + string.digits


def generate_random_string(length):
    # Define the possible characters: a-z, A-Z, 0-9

    # Use random.choices to select characters and ''.join to form the string
    random_string = ''.join(random.choices(characters, k=length))
    return random_string


def create_account(client: httpx.Client, auth_addr: str, username: str, email: str, password: str) -> str:
    resp = client.post(
        f"{auth_addr}/api/v1/account",
        json={"username": username, "email": email, "password": password},
    )
    if resp.status_code != 200:
        raise RuntimeError(
            f"create_account failed ({resp.status_code}): {resp.text}")
    account_id = resp.json()["account_id"]
    print(f"  created  : {username} ({email}) → {account_id}")
    return account_id


def activate_account(client: httpx.Client, auth_addr: str, account_id: str) -> None:
    resp = client.post(f"{auth_addr}/api/v1/account/activate/{account_id}")
    if resp.status_code != 200:
        raise RuntimeError(
            f"activate_account failed ({resp.status_code}): {resp.text}")
    print(f"  activated: {account_id}")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--auth", "-a", default=AUTH_DEFAULT,
                        help=f"Auth service base URL (default: {AUTH_DEFAULT})")
    args = parser.parse_args()

    with httpx.Client(timeout=30.0) as client:
        for u in SEED_USERS:
            print(f"\nSeeding '{u['username']}' ...")
            try:
                account_id = create_account(
                    client, args.auth, u["username"], u["email"], generate_random_string(8))
                activate_account(client, args.auth, account_id)
            except RuntimeError as e:
                print(f"  skipped: {e}")

    print("\nDone.")


if __name__ == "__main__":
    main()

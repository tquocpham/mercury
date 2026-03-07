"""
Test rate limiting on gateway /api/v1/hc/ping and /api/v1/hc/auth.

Rate limit rules (from gateway main.go):
  LimitAnonUsers(rdb, 2)          -- anonymous: 2 req/s by IP
  LimitUsersByRoles(rdb, 10, "User") -- authenticated non-"User" role: 10 req/s by username
                                      -- "User" role: skipped (no limit from this rule)

Endpoints:
  GET /api/v1/hc/ping  -- no auth required
  GET /api/v1/hc/auth  -- requires valid session cookie (UseAuth middleware)

Usage:
    # Anonymous test only
    python test_ratelimit.py

    # With authenticated tests
    python test_ratelimit.py --user alice --password secret

    # Customise burst size
    python test_ratelimit.py --user alice --password secret --burst 15
"""
import argparse
import base64
import json
import time

import httpx


GATEWAY_DEFAULT = "http://localhost:9001"
AUTH_DEFAULT = "https://localhost:9005"


def b64_decode(s: str) -> bytes:
    s += "=" * (-len(s) % 4)
    return base64.urlsafe_b64decode(s)


def decode_jwt(token: str) -> dict:
    parts = token.split(".")
    if len(parts) != 3:
        raise ValueError("not a valid JWT")
    return json.loads(b64_decode(parts[1]))


def signin(auth_addr: str, username: str, password: str) -> str:
    resp = httpx.post(
        f"{auth_addr}/api/v1/auth",
        json={"credentials": {"username": username, "password": password}},
    )
    if resp.status_code != 200:
        raise RuntimeError(f"sign-in failed ({resp.status_code}): {resp.text}")
    return resp.json()["token"]


def run_burst(url: str, n: int, cookies: dict | None = None) -> None:
    """Fire n requests as fast as possible and print per-request results."""
    ok = 0
    limited = 0
    other = 0
    first_limited = None

    with httpx.Client(cookies=cookies or {}) as client:
        for i in range(1, n + 1):
            resp = client.get(url)
            code = resp.status_code
            if 200 <= code < 300:
                ok += 1
                tag = f"OK     ({code})"
            elif code in (401, 429):
                limited += 1
                if first_limited is None:
                    first_limited = i
                tag = f"DENIED ({code})"
            else:
                other += 1
                tag = f"OTHER  ({code})"
            print(f"  [{i:3d}] {tag}")

    print(f"\n  Results: {ok} ok | {limited} rate-limited | {other} other")
    if first_limited:
        print(f"  First denial on request #{first_limited}")
    else:
        print("  No requests were rate-limited")


def section(title: str) -> None:
    print(f"\n{'=' * 55}")
    print(f"  {title}")
    print("=" * 55)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--gateway", "-g", default=GATEWAY_DEFAULT,
                        help=f"Gateway base URL (default: {GATEWAY_DEFAULT})")
    parser.add_argument("--auth", "-a", default=AUTH_DEFAULT,
                        help=f"Auth service base URL (default: {AUTH_DEFAULT})")
    parser.add_argument(
        "--user", "-u", help="Username for authenticated tests")
    parser.add_argument("--password", "-p",
                        help="Password for authenticated tests")
    parser.add_argument("--burst", "-n", type=int, default=8,
                        help="Requests to send per test (default: 8)")
    args = parser.parse_args()

    ping_url = f"{args.gateway}/api/v1/hc/ping"
    auth_url = f"{args.gateway}/api/v1/hc/auth"

    # ------------------------------------------------------------------ #
    # TEST 1 — anonymous /ping                                            #
    # Expected: first 2 succeed, rest are denied                         #
    # ------------------------------------------------------------------ #
    section(f"TEST 1 · anonymous  GET /api/v1/hc/ping  (limit: 2 req/s by IP)")
    run_burst(ping_url, args.burst)

    time.sleep(2)  # let the 1-second window expire

    if not args.user or not args.password:
        print("\n[skipping authenticated tests — pass --user / --password]\n")
        return

    # ------------------------------------------------------------------ #
    # Sign in                                                             #
    # ------------------------------------------------------------------ #
    print(f"\nSigning in as '{args.user}' @ {args.auth} ...")
    token = signin(args.auth, args.user, args.password)
    claims = decode_jwt(token)
    role = claims.get("role")
    print(f"  username : {claims.get('username')}")
    print(f"  role     : {role!r}")
    if role == "User":
        print(
            "  note     : 'User' role is skipped by LimitUsersByRoles — no auth-based cap")
    else:
        print("  note     : non-'User' role — limit 10 req/s by username")

    cookies = {"session": token}

    time.sleep(1)

    # ------------------------------------------------------------------ #
    # TEST 2 — authenticated /ping                                        #
    # Expected (non-User role): first 10 succeed, rest denied            #
    # Expected (User role): all pass through (rule skipped)              #
    # ------------------------------------------------------------------ #
    section(f"TEST 2 · authenticated  GET /api/v1/hc/ping  (limit: 10 req/s for non-User role)")
    run_burst(ping_url, args.burst, cookies=cookies)

    time.sleep(2)

    # ------------------------------------------------------------------ #
    # TEST 3 — authenticated /auth                                        #
    # Same rate rules as /ping but also requires a valid session cookie.  #
    # An invalid or missing cookie → 401 before rate limiting applies.   #
    # ------------------------------------------------------------------ #
    section(f"TEST 3 · authenticated  GET /api/v1/hc/auth  (requires session cookie + same limits)")
    run_burst(auth_url, args.burst, cookies=cookies)

    print()


if __name__ == "__main__":
    main()

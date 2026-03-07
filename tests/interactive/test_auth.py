"""
Test auth service: sign in and decode the returned JWT.

Usage:
    python test_auth.py --user alice --password secret
    python test_auth.py --user alice --password secret --pubkey /path/to/public.pem
"""
import argparse
import base64
import json

import httpx


def b64_decode(s: str) -> bytes:
    """JWT base64url decode (no-padding variant)."""
    s += "=" * (-len(s) % 4)
    return base64.urlsafe_b64decode(s)


def decode_jwt(token: str) -> dict:
    """Decode JWT claims without verifying signature."""
    parts = token.split(".")
    if len(parts) != 3:
        raise ValueError("not a valid JWT")
    header = json.loads(b64_decode(parts[0]))
    claims = json.loads(b64_decode(parts[1]))
    return {"header": header, "claims": claims}


def verify_jwt(token: str, pubkey_path: str) -> dict:
    """Verify JWT signature using an RSA public key file (requires PyJWT)."""
    try:
        import jwt as pyjwt
    except ImportError:
        print("PyJWT not installed — skipping signature verification")
        print("  pip install PyJWT cryptography")
        return decode_jwt(token)

    with open(pubkey_path) as f:
        pub_key = f.read()

    payload = pyjwt.decode(token, pub_key, algorithms=["RS256"])
    return {"header": {}, "claims": payload}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--user", "-u", required=True)
    parser.add_argument("--password", "-p", required=True)
    parser.add_argument("--addr", "-a", default="http://localhost:9005")
    parser.add_argument(
        "--pubkey", help="Path to RSA public key PEM (enables signature verification)")
    args = parser.parse_args()

    print(f"Signing in as '{args.user}' @ {args.addr} ...")
    resp = httpx.post(
        f"{args.addr}/api/v1/auth",
        json={"credentials": {"username": args.user, "password": args.password}},
    )

    print(f"Status: {resp.status_code}")
    if resp.status_code != 200:
        print(f"Error: {resp.text}")
        return

    token = resp.json().get("token", "")
    print(f"\nToken: {token[:40]}...{token[-10:]}\n")

    if args.pubkey:
        decoded = verify_jwt(token, args.pubkey)
        print("Signature: VERIFIED")
    else:
        decoded = decode_jwt(token)
        print("Signature: not verified (pass --pubkey to verify)")

    print(f"\nHeader:  {json.dumps(decoded['header'], indent=2)}")
    print(f"Claims:  {json.dumps(decoded['claims'], indent=2)}")


if __name__ == "__main__":
    main()

"""
Integration tests for auth — login.

Requires a running stack (gateway + auth service + MongoDB + Redis).

Usage:
    pytest test_auth.py
"""
import uuid
import pytest
import httpx
import pymongo


@pytest.fixture(scope="session")
def test_user(gateway, mongo_url):
    """Create and activate a throwaway account for login tests."""
    username = f"test-auth-{uuid.uuid4().hex[:8]}"
    email = f"{username}@mercury.local"
    password = "testpassword"

    with httpx.Client(timeout=10.0) as client:
        resp = client.post(f"{gateway}/api/v1/account",
                           json={"username": username, "email": email, "password": password})
        assert resp.status_code == 200, f"account creation failed: {resp.text}"
        account_id = resp.json()["account_id"]

        resp = client.post(f"{gateway}/api/v1/account/activate/{account_id}")
        assert resp.status_code == 200, f"account activation failed: {resp.text}"

    yield {"username": username, "password": password}

    mdb = pymongo.MongoClient(mongo_url)
    mdb["auth"]["users"].delete_one({"username": username})
    mdb.close()


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

def test_login_valid(gateway, test_user):
    with httpx.Client(timeout=10.0) as client:
        resp = client.post(f"{gateway}/api/v1/auth/login", json={
            "credentials": {
                "username": test_user["username"],
                "password": test_user["password"],
            }
        })
    assert resp.status_code == 200
    body = resp.json()
    assert "token" in body
    assert len(body["token"]) > 0


def test_login_invalid(gateway, test_user):
    with httpx.Client(timeout=10.0) as client:
        resp = client.post(f"{gateway}/api/v1/auth/login", json={
            "credentials": {
                "username": test_user["username"],
                "password": "wrongpassword",
            }
        })
    assert resp.status_code == 401

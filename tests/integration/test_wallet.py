"""
Integration tests for the wallet service via the private gateway.

Requires a running stack (gatewaypriv + wallet service + MongoDB + RabbitMQ).

Usage:
    pytest test_wallet.py
"""
import uuid
import pytest
import httpx
from ulid import ULID


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def new_order_id() -> str:
    return str(ULID())


def new_player_id(tag: str) -> str:
    return f"wallet-test-{tag}-{uuid.uuid4().hex[:8]}"


def add_currency(
    client: httpx.Client,
    gatewaypriv: str,
    player_id: str,
    currency_id: str,
    amount: int,
    order_id: str,
) -> dict:
    resp = client.post(f"{gatewaypriv}/api/v1/wallet/add_currency", json={
        "player_id":   player_id,
        "currency_id": currency_id,
        "amount":      amount,
        "order_id":    order_id,
    })
    assert resp.status_code == 200, f"add_currency failed ({resp.status_code}): {resp.text}"
    return resp.json()


def currency_amount(wallet_resp: dict, currency_id: str) -> int:
    for c in wallet_resp.get("currencies", []):
        if c["currency_type"] == currency_id:
            return c["amount"]
    return 0


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def player(mongo_client):
    """Yields a unique player_id and cleans up wallet data after the test."""
    player_id = new_player_id("test")
    yield player_id
    mongo_client["wallet"]["wallets"].delete_one({"player_id": player_id})
    mongo_client["wallet"]["processed_orders"].delete_many({"player_id": player_id})


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

def test_grant_new_player(gatewaypriv, player):
    with httpx.Client(timeout=30.0) as client:
        resp = add_currency(client, gatewaypriv, player, "gold", 100, new_order_id())

    assert resp["player_id"] == player
    assert currency_amount(resp, "gold") == 100


def test_accumulate_currency(gatewaypriv, player):
    grants = [50, 75, 25]
    with httpx.Client(timeout=30.0) as client:
        for amount in grants:
            resp = add_currency(client, gatewaypriv, player, "gems", amount, new_order_id())

    assert currency_amount(resp, "gems") == sum(grants)


def test_idempotency(gatewaypriv, player):
    order_id = new_order_id()
    with httpx.Client(timeout=30.0) as client:
        resp1 = add_currency(client, gatewaypriv, player, "coins", 200, order_id)
        resp2 = add_currency(client, gatewaypriv, player, "coins", 200, order_id)

    assert currency_amount(resp1, "coins") == 200
    assert currency_amount(resp2, "coins") == 200


def test_multiple_currency_types(gatewaypriv, player):
    grants = [("gold", 500), ("gems", 30), ("tickets", 3)]
    with httpx.Client(timeout=30.0) as client:
        for currency_id, amount in grants:
            resp = add_currency(client, gatewaypriv, player, currency_id, amount, new_order_id())

    for currency_id, expected in grants:
        assert currency_amount(resp, currency_id) == expected

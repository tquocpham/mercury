"""
Integration test: two players trading currency via the trade service.

Flow:
  1. Pre-fund player1 with gold and player2 with gems.
  2. Create a bilateral trade — player2 receives gold, player1 receives gems.
  3. Poll trade status until COMPLETED (tradecourier delivers the grants).
  4. Verify both wallets reflect the received currency.

Requires: gatewaypriv + trade + wallet + tradecourier + MongoDB + RabbitMQ.

Usage:
    pytest test_trade_currency.py
"""
import time
import uuid
import pytest
import httpx
from ulid import ULID

POLL_INTERVAL = 1.0   # seconds between status checks
POLL_TIMEOUT  = 30.0  # seconds before giving up


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def new_order_id() -> str:
    return str(ULID())


def new_player_id(tag: str) -> str:
    return f"trade-test-{tag}-{uuid.uuid4().hex[:8]}"


def add_currency(client: httpx.Client, gatewaypriv: str,
                 player_id: str, currency_id: str, amount: int, order_id: str) -> dict:
    resp = client.post(f"{gatewaypriv}/api/v1/wallet/add_currency", json={
        "player_id":   player_id,
        "currency_id": currency_id,
        "amount":      amount,
        "order_id":    order_id,
    })
    assert resp.status_code == 200, f"add_currency failed ({resp.status_code}): {resp.text}"
    return resp.json()


def get_wallet(client: httpx.Client, gatewaypriv: str, player_id: str) -> dict:
    url = f"{gatewaypriv}/api/v1/wallet/wallet/{player_id}"
    print(url)
    resp = client.get(url)
    assert resp.status_code == 200, f"get_wallet failed ({resp.status_code}): {resp.text}"
    return resp.json()


def create_trade(client: httpx.Client, gatewaypriv: str,
                 order_id: str, initiator_id: str, grants: list[dict]) -> dict:
    resp = client.post(f"{gatewaypriv}/api/v1/trade/dispatch", json={
        "order_id":     order_id,
        "initiator_id": initiator_id,
        "grants":       grants,
    })
    assert resp.status_code == 200, f"create_trade failed ({resp.status_code}): {resp.text}"
    return resp.json()


def get_trade_status(client: httpx.Client, gatewaypriv: str, order_id: str) -> dict:
    resp = client.get(f"{gatewaypriv}/api/v1/trade/status/{order_id}")
    assert resp.status_code == 200, f"get_trade_status failed ({resp.status_code}): {resp.text}"
    return resp.json()

def wait_for_completed(client: httpx.Client, gatewaypriv: str, order_id: str) -> str:
    """Poll trade status until COMPLETED or FAILED, or timeout. Returns final status."""
    deadline = time.monotonic() + POLL_TIMEOUT
    while time.monotonic() < deadline:
        status = get_trade_status(client, gatewaypriv, order_id)["status"]
        if status in ("COMPLETED", "FAILED"):
            return status
        time.sleep(POLL_INTERVAL)
    return "TIMEOUT"


def currency_amount(wallet_resp: dict, currency_id: str) -> int:
    for c in wallet_resp.get("currencies", []):
        if c["currency_type"] == currency_id:
            return c["amount"]
    return 0


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def two_players(mongo_client):
    """Yields (player1_id, player2_id) and cleans up wallets and trade outbox after."""
    p1 = new_player_id("p1")
    p2 = new_player_id("p2")

    yield p1, p2

    # for pid in (p1, p2):
    #     mongo_client["wallet"]["wallets"].delete_one({"player_id": pid})
    #     mongo_client["wallet"]["processed_orders"].delete_many({"player_id": pid})
    # mongo_client["trade"]["outbox"].delete_many({"initiator_id": p1})


# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------

def test_bilateral_currency_trade(gatewaypriv, two_players):
    """
    Player1 gives gold to player2; player2 gives gems to player1.
    Verifies both balances are updated after the trade completes.
    """
    player1, player2 = two_players

    with httpx.Client(timeout=30.0) as client:
        # Pre-fund both players
        add_currency(client, gatewaypriv, player1, "gold", 1000, new_order_id())
        add_currency(client, gatewaypriv, player2, "gems", 500,  new_order_id())

        # Create bilateral trade: player2 gets 100 gold, player1 gets 50 gems
        order_id = new_order_id()
        trade_resp = create_trade(client, gatewaypriv, order_id, player1, [
            {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
            {"player_id": player1, "type": "CURRENCY", "target_id": "gems", "amount": 50},
        ])
        assert trade_resp["order_id"] == order_id

        # Wait for tradecourier to deliver the grants
        final_status = wait_for_completed(client, gatewaypriv, order_id)
        assert final_status == "COMPLETED", f"trade did not complete — status: {final_status}"

        # Verify player2 received gold
        w2 = get_wallet(client, gatewaypriv, player2)
        assert currency_amount(w2, "gold") == 100

        # Verify player1 received gems
        w1 = get_wallet(client, gatewaypriv, player1)
        assert currency_amount(w1, "gems") == 50

"""
Integration tests: trade draft/lock/unlock flow.

Tests the collaborative trade builder where players draft grants,
sign to accept, and can unlock to propose changes.

Requires: gatewaypriv + trade + MongoDB + RabbitMQ.

Usage:
    pytest test_trade.py
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
    return f"trade-test-{tag}-{uuid.uuid4().hex[:8]}"


def draft_trade(client: httpx.Client, gatewaypriv: str, order_id: str,
                player_id: str, initiator_id: str, contracting_parties: list[str],
                transaction_id: str = "", grants: list[dict] = None) -> dict:
    resp = client.post(f"{gatewaypriv}/api/v1/trade/draft", json={
        "order_id":             order_id,
        "player_id":            player_id,
        "initiator_id":         initiator_id,
        "contracting_parties":  contracting_parties,
        "transaction_id":       transaction_id,
        "grants":               grants or [],
    })
    assert resp.status_code == 200, f"draft_trade failed ({resp.status_code}): {resp.text}"
    return resp.json()


def lock_trade(client: httpx.Client, gatewaypriv: str,
               order_id: str, player_id: str, transaction_id: str) -> dict:
    resp = client.post(f"{gatewaypriv}/api/v1/trade/lock", json={
        "order_id":       order_id,
        "player_id":      player_id,
        "transaction_id": transaction_id,
    })
    assert resp.status_code == 200, f"lock_trade failed ({resp.status_code}): {resp.text}"
    return resp.json()


def unlock_trade(client: httpx.Client, gatewaypriv: str,
                 order_id: str, player_id: str, transaction_id: str) -> dict:
    resp = client.post(f"{gatewaypriv}/api/v1/trade/unlock", json={
        "order_id":       order_id,
        "player_id":      player_id,
        "transaction_id": transaction_id,
    })
    assert resp.status_code == 200, f"unlock_trade failed ({resp.status_code}): {resp.text}"
    return resp.json()


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def two_players():
    p1 = new_player_id("p1")
    p2 = new_player_id("p2")
    yield p1, p2


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

def test_draft_trade_creates_new(gatewaypriv, two_players):
    """First DraftTrade call creates the outbox and sets the initiator's grants."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])

        assert resp["order_id"] == order_id
        assert resp["transaction_id"] != ""
        assert player1 in resp["grants_by_player"]
        assert len(resp["grants_by_player"][player1]) == 1
        assert resp["signatures"] == []


def test_draft_trade_second_player_adds_grants(gatewaypriv, two_players):
    """Player2's draft adds their grants without overwriting player1's."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp1 = draft_trade(client, gatewaypriv, order_id, player1, player1,
                            [player1, player2],
                            grants=[
                                {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                            ])

        resp2 = draft_trade(client, gatewaypriv, order_id, player2, player1,
                            [player1, player2],
                            transaction_id=resp1["transaction_id"],
                            grants=[
                                {"player_id": player1, "type": "CURRENCY", "target_id": "gems", "amount": 50},
                            ])

        assert player1 in resp2["grants_by_player"]
        assert player2 in resp2["grants_by_player"]
        assert resp2["grants_by_player"][player1][0]["target_id"] == "gold"
        assert resp2["grants_by_player"][player2][0]["target_id"] == "gems"


def test_draft_trade_stale_transaction_id_conflicts(gatewaypriv, two_players):
    """Using a stale transaction_id returns 409 Conflict."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp1 = draft_trade(client, gatewaypriv, order_id, player1, player1,
                            [player1, player2],
                            grants=[
                                {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                            ])
        stale_id = resp1["transaction_id"]

        # Advance the transaction_id
        draft_trade(client, gatewaypriv, order_id, player1, player1,
                    [player1, player2],
                    transaction_id=stale_id,
                    grants=[
                        {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 200},
                    ])

        # Re-using the stale id should now conflict
        resp = client.post(f"{gatewaypriv}/api/v1/trade/draft", json={
            "order_id":    order_id,
            "player_id":   player1,
            "initiator_id": player1,
            "transaction_id": stale_id,
            "grants": [{"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 300}],
        })
        assert resp.status_code == 409, f"expected 409, got {resp.status_code}: {resp.text}"


def test_lock_trade_adds_signature(gatewaypriv, two_players):
    """A player locking the trade adds their signature; status stays DRAFT until all sign."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])

        lock_resp = lock_trade(client, gatewaypriv, order_id, player1, resp["transaction_id"])

        assert player1 in lock_resp["signatures"]
        assert lock_resp["status"] == "DRAFT"


def test_lock_prevents_grant_changes(gatewaypriv, two_players):
    """Once any player has signed, grant updates are rejected with 409."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])

        lock_resp = lock_trade(client, gatewaypriv, order_id, player1, resp["transaction_id"])

        resp = client.post(f"{gatewaypriv}/api/v1/trade/draft", json={
            "order_id":       order_id,
            "player_id":      player1,
            "initiator_id":   player1,
            "transaction_id": lock_resp["transaction_id"],
            "grants": [{"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 999}],
        })
        assert resp.status_code == 409, f"expected 409, got {resp.status_code}: {resp.text}"


def test_unlock_clears_signatures_and_allows_changes(gatewaypriv, two_players):
    """After unlock, signatures are cleared and grant updates are accepted again."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])

        lock_resp = lock_trade(client, gatewaypriv, order_id, player1, resp["transaction_id"])
        unlock_resp = unlock_trade(client, gatewaypriv, order_id, player1, lock_resp["transaction_id"])

        assert unlock_resp["signatures"] == []

        updated = draft_trade(client, gatewaypriv, order_id, player1, player1,
                              [player1, player2],
                              transaction_id=unlock_resp["transaction_id"],
                              grants=[
                                  {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 200},
                              ])
        assert updated["grants_by_player"][player1][0]["amount"] == 200


def test_all_signed_transitions_to_pending(gatewaypriv, two_players):
    """When all contracting parties sign, the trade transitions to PENDING."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])

        lock_resp1 = lock_trade(client, gatewaypriv, order_id, player1, resp["transaction_id"])
        assert lock_resp1["status"] == "DRAFT"

        lock_resp2 = lock_trade(client, gatewaypriv, order_id, player2, lock_resp1["transaction_id"])
        assert lock_resp2["status"] == "PENDING"
        assert set(lock_resp2["signatures"]) == {player1, player2}

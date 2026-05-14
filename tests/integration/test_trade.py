"""
Integration tests: trade draft/lock/unlock flow.

Tests the collaborative trade builder where players draft grants,
sign to accept, and can unlock to propose changes.

Requires: gatewaypriv + trade + MongoDB + RabbitMQ.

Usage:
    pytest test_trade.py
"""
import time
import uuid
import pytest
import httpx
from ulid import ULID

POLL_INTERVAL = 1.0
POLL_TIMEOUT  = 30.0


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
    resp = client.get(f"{gatewaypriv}/api/v1/wallet/wallet/{player_id}")
    assert resp.status_code == 200, f"get_wallet failed ({resp.status_code}): {resp.text}"
    return resp.json()


def get_trade_status(client: httpx.Client, gatewaypriv: str, order_id: str) -> dict:
    resp = client.get(f"{gatewaypriv}/api/v1/trade/status/{order_id}")
    assert resp.status_code == 200, f"get_trade_status failed ({resp.status_code}): {resp.text}"
    return resp.json()


def wait_for_completed(client: httpx.Client, gatewaypriv: str, order_id: str) -> str:
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


def test_lock_stale_transaction_id_conflicts(gatewaypriv, two_players):
    """Locking with a stale transaction_id returns 409."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])
        stale_id = resp["transaction_id"]

        # Advance the transaction_id
        draft_trade(client, gatewaypriv, order_id, player1, player1,
                    [player1, player2],
                    transaction_id=stale_id,
                    grants=[
                        {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 200},
                    ])

        resp = client.post(f"{gatewaypriv}/api/v1/trade/lock", json={
            "order_id":       order_id,
            "player_id":      player1,
            "transaction_id": stale_id,
        })
        assert resp.status_code == 409, f"expected 409, got {resp.status_code}: {resp.text}"


def test_unlock_stale_transaction_id_conflicts(gatewaypriv, two_players):
    """Unlocking with a stale transaction_id returns 409."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])

        lock_resp = lock_trade(client, gatewaypriv, order_id, player1, resp["transaction_id"])
        stale_id = lock_resp["transaction_id"]

        # Advance the transaction_id by unlocking
        unlock_trade(client, gatewaypriv, order_id, player1, stale_id)

        resp = client.post(f"{gatewaypriv}/api/v1/trade/unlock", json={
            "order_id":       order_id,
            "player_id":      player1,
            "transaction_id": stale_id,
        })
        assert resp.status_code == 409, f"expected 409, got {resp.status_code}: {resp.text}"


def test_non_contracting_party_cannot_lock(gatewaypriv, two_players):
    """A player not in contracting_parties cannot lock the trade."""
    player1, player2 = two_players
    outsider = new_player_id("outsider")
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])

        resp = client.post(f"{gatewaypriv}/api/v1/trade/lock", json={
            "order_id":       order_id,
            "player_id":      outsider,
            "transaction_id": resp["transaction_id"],
        })
        assert resp.status_code == 409, f"expected 409, got {resp.status_code}: {resp.text}"


def test_non_contracting_party_cannot_unlock(gatewaypriv, two_players):
    """A player not in contracting_parties cannot unlock the trade."""
    player1, player2 = two_players
    outsider = new_player_id("outsider")
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])

        lock_resp = lock_trade(client, gatewaypriv, order_id, player1, resp["transaction_id"])

        resp = client.post(f"{gatewaypriv}/api/v1/trade/unlock", json={
            "order_id":       order_id,
            "player_id":      outsider,
            "transaction_id": lock_resp["transaction_id"],
        })
        assert resp.status_code == 409, f"expected 409, got {resp.status_code}: {resp.text}"


def test_cannot_lock_pending_trade(gatewaypriv, two_players):
    """Once a trade is PENDING, further lock attempts are rejected."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])

        lock_resp1 = lock_trade(client, gatewaypriv, order_id, player1, resp["transaction_id"])
        lock_resp2 = lock_trade(client, gatewaypriv, order_id, player2, lock_resp1["transaction_id"])
        assert lock_resp2["status"] == "PENDING"

        resp = client.post(f"{gatewaypriv}/api/v1/trade/lock", json={
            "order_id":       order_id,
            "player_id":      player1,
            "transaction_id": lock_resp2["transaction_id"],
        })
        assert resp.status_code == 409, f"expected 409, got {resp.status_code}: {resp.text}"


def test_cannot_unlock_pending_trade(gatewaypriv, two_players):
    """Once a trade is PENDING, unlock attempts are rejected."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = draft_trade(client, gatewaypriv, order_id, player1, player1,
                           [player1, player2],
                           grants=[
                               {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                           ])

        lock_resp1 = lock_trade(client, gatewaypriv, order_id, player1, resp["transaction_id"])
        lock_resp2 = lock_trade(client, gatewaypriv, order_id, player2, lock_resp1["transaction_id"])
        assert lock_resp2["status"] == "PENDING"

        resp = client.post(f"{gatewaypriv}/api/v1/trade/unlock", json={
            "order_id":       order_id,
            "player_id":      player1,
            "transaction_id": lock_resp2["transaction_id"],
        })
        assert resp.status_code == 409, f"expected 409, got {resp.status_code}: {resp.text}"


def test_any_party_can_unlock(gatewaypriv, two_players):
    """Any contracting party can unlock, not just the one who signed."""
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

        # Player2 unlocks player1's signature
        unlock_resp = unlock_trade(client, gatewaypriv, order_id, player2, lock_resp["transaction_id"])
        assert unlock_resp["signatures"] == []


def test_draft_missing_player_id(gatewaypriv, two_players):
    """DraftTrade without player_id returns 400."""
    player1, player2 = two_players
    order_id = new_order_id()

    with httpx.Client(timeout=10.0) as client:
        resp = client.post(f"{gatewaypriv}/api/v1/trade/draft", json={
            "order_id":            order_id,
            "initiator_id":        player1,
            "contracting_parties": [player1, player2],
            "grants":              [],
        })
        assert resp.status_code == 400, f"expected 400, got {resp.status_code}: {resp.text}"


def test_draft_invalid_order_id(gatewaypriv, two_players):
    """DraftTrade with a non-ULID order_id returns 400."""
    player1, player2 = two_players

    with httpx.Client(timeout=10.0) as client:
        resp = client.post(f"{gatewaypriv}/api/v1/trade/draft", json={
            "order_id":            "not-a-valid-ulid",
            "player_id":           player1,
            "initiator_id":        player1,
            "contracting_parties": [player1, player2],
            "grants":              [],
        })
        assert resp.status_code == 400, f"expected 400, got {resp.status_code}: {resp.text}"


def test_bilateral_draft_trade_delivers_grants(gatewaypriv, two_players):
    """
    Full end-to-end: both players draft grants, both sign, tradecourier
    delivers, and both wallets reflect the received currency.
    """
    player1, player2 = two_players

    with httpx.Client(timeout=30.0) as client:
        # Pre-fund both players
        add_currency(client, gatewaypriv, player1, "gold", 1000, new_order_id())
        add_currency(client, gatewaypriv, player2, "gems", 500,  new_order_id())

        order_id = new_order_id()

        # Player1 drafts: player2 receives 100 gold
        resp1 = draft_trade(client, gatewaypriv, order_id, player1, player1,
                            [player1, player2],
                            grants=[
                                {"player_id": player2, "type": "CURRENCY", "target_id": "gold", "amount": 100},
                            ])

        # Player2 drafts: player1 receives 50 gems
        resp2 = draft_trade(client, gatewaypriv, order_id, player2, player1,
                            [player1, player2],
                            transaction_id=resp1["transaction_id"],
                            grants=[
                                {"player_id": player1, "type": "CURRENCY", "target_id": "gems", "amount": 50},
                            ])

        # Both players sign
        lock_resp1 = lock_trade(client, gatewaypriv, order_id, player1, resp2["transaction_id"])
        assert lock_resp1["status"] == "DRAFT"

        lock_resp2 = lock_trade(client, gatewaypriv, order_id, player2, lock_resp1["transaction_id"])
        assert lock_resp2["status"] == "PENDING"

        # Wait for tradecourier to deliver
        final_status = wait_for_completed(client, gatewaypriv, order_id)
        assert final_status == "COMPLETED", f"trade did not complete — status: {final_status}"

        # Verify player2 received gold
        w2 = get_wallet(client, gatewaypriv, player2)
        assert currency_amount(w2, "gold") == 100

        # Verify player1 received gems
        w1 = get_wallet(client, gatewaypriv, player1)
        assert currency_amount(w1, "gems") == 50

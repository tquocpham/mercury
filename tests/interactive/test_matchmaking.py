"""
Integration test for the matchmaking pipeline.

Phase 1 — Assignment:
  Seeds game servers and parties via the gateway HTTP API, polls MongoDB
  until the mmsolver assigns all parties.

Phase 2 — Notifications (optional, --ws):
  Creates real user accounts, opens WebSocket connections to the subscriber,
  queues the players as a party, and verifies every player receives a
  Matchmake notification over their WebSocket connection.

Usage:
    python test_matchmaking.py
    python test_matchmaking.py --ws
    python test_matchmaking.py --gateway http://localhost:9001 \\
                               --subscriber http://localhost:9004 \\
                               --mongo mongodb://root:root@localhost:27017
"""
import argparse
import asyncio
import base64
import json
import string
import random
import time

import httpx
import pymongo
import websockets

GATEWAY_DEFAULT     = "http://localhost:9001"
GATEWAYPRIV_DEFAULT = "http://localhost:9002"
SUBSCRIBER_DEFAULT  = "http://localhost:9004"
MONGO_DEFAULT       = "mongodb://root:root@localhost:27017"
POLL_INTERVAL      = 1.0    # seconds between MongoDB checks
POLL_TIMEOUT       = 30.0   # seconds before giving up
WS_TIMEOUT         = 30.0   # seconds to wait for WS notification

# ---------------------------------------------------------------------------
# Phase 1 — static test data (fake UUIDs, no auth required)
# ---------------------------------------------------------------------------

GAME_SERVERS = [
    {"server_id": "gs-test-1", "ip_address": "10.0.0.1", "port": 7001, "capacity": 4},
    {"server_id": "gs-test-2", "ip_address": "10.0.0.2", "port": 7002, "capacity": 6},
]

PARTIES = [
    {"party_id": "party-test-alpha", "player_ids": ["uid-a1", "uid-a2"]},
    {"party_id": "party-test-beta",  "player_ids": ["uid-b1", "uid-b2", "uid-b3"]},
    {"party_id": "party-test-gamma", "player_ids": ["uid-c1"]},
    {"party_id": "party-test-delta", "player_ids": ["uid-d1", "uid-d2", "uid-d3"]},
]

# ---------------------------------------------------------------------------
# Phase 2 — real user accounts for WebSocket notification test
# ---------------------------------------------------------------------------

WS_PLAYERS = [
    {"username": "mm-ws-test-1", "email": "mm-ws-test-1@mercury.local", "password": "testpassword"},
    {"username": "mm-ws-test-2", "email": "mm-ws-test-2@mercury.local", "password": "testpassword"},
    {"username": "mm-ws-test-3", "email": "mm-ws-test-3@mercury.local", "password": "testpassword"},
]
WS_SERVER = {"server_id": "gs-ws-test", "ip_address": "10.0.1.1", "port": 7010, "capacity": len(WS_PLAYERS)}
WS_PARTY_ID = "party-ws-test"


# ---------------------------------------------------------------------------
# Auth helpers
# ---------------------------------------------------------------------------

def _rand_suffix(n=6) -> str:
    return "".join(random.choices(string.ascii_lowercase + string.digits, k=n))


def ensure_account(client: httpx.Client, gateway: str, username: str, email: str, password: str) -> None:
    """Create + activate an account, ignoring errors if it already exists."""
    resp = client.post(f"{gateway}/api/v1/account",
                       json={"username": username, "email": email, "password": password})
    if resp.status_code == 200:
        account_id = resp.json()["account_id"]
        client.post(f"{gateway}/api/v1/account/activate/{account_id}")
        print(f"  created account: {username} ({account_id})")
    else:
        print(f"  account exists or error for {username}: {resp.status_code} — continuing")


def login(client: httpx.Client, gateway: str, username: str, password: str) -> str:
    """Login and return the JWT token string."""
    resp = client.post(f"{gateway}/api/v1/auth/login",
                       json={"credentials": {"username": username, "password": password}})
    if resp.status_code != 200:
        raise RuntimeError(f"login failed for {username} ({resp.status_code}): {resp.text}")
    return resp.json()["token"]


def decode_user_id(token: str) -> str:
    """Extract user_id from JWT payload without verifying signature."""
    payload_b64 = token.split(".")[1]
    # Pad to a multiple of 4
    payload_b64 += "=" * (-len(payload_b64) % 4)
    payload = json.loads(base64.urlsafe_b64decode(payload_b64))
    return payload["user_id"]


# ---------------------------------------------------------------------------
# Seeding helpers
# ---------------------------------------------------------------------------

def register_server(client: httpx.Client, gatewaypriv: str, gs: dict) -> None:
    resp = client.post(f"{gatewaypriv}/api/v1/gs/register", json={
        "server_id":  gs["server_id"],
        "ip_address": gs["ip_address"],
        "port":       gs["port"],
        "capacity":   gs["capacity"],
    })
    if resp.status_code != 200:
        raise RuntimeError(f"register_server failed ({resp.status_code}): {resp.text}")
    print(f"  registered server: {gs['server_id']} capacity={gs['capacity']}")


def queue_party(client: httpx.Client, gateway: str, party: dict) -> str:
    resp = client.post(f"{gateway}/api/v1/mm/join/party", json={
        "party_id":   party["party_id"],
        "player_ids": party["player_ids"],
    })
    if resp.status_code != 200:
        raise RuntimeError(f"queue_party failed ({resp.status_code}): {resp.text}")
    queue_id = resp.json().get("party_id", party["party_id"])
    print(f"  queued party: {party['party_id']} ({len(party['player_ids'])} players) → queue_id={queue_id}")
    return queue_id


# ---------------------------------------------------------------------------
# Phase 1 verification — API polling
# ---------------------------------------------------------------------------

def get_queue(client: httpx.Client, gateway: str, party_id: str) -> dict:
    resp = client.get(f"{gateway}/api/v1/mm/join/party/{party_id}")
    if resp.status_code != 200:
        raise RuntimeError(f"get_queue failed ({resp.status_code}): {resp.text}")
    return resp.json()


def wait_for_assignments(gateway: str, party_ids: list) -> bool:
    print(f"\nPolling gateway for assignments (timeout={POLL_TIMEOUT}s) …")
    with httpx.Client(timeout=10.0) as client:
        deadline = time.monotonic() + POLL_TIMEOUT
        while time.monotonic() < deadline:
            queues = {pid: get_queue(client, gateway, pid) for pid in party_ids}
            assigned = [pid for pid in party_ids if queues[pid].get("status") == "assigned"]
            pending  = [pid for pid in party_ids if queues[pid].get("status") == "pending"]

            print(f"  assigned={len(assigned)}/{len(party_ids)}  pending={len(pending)}")

            if len(assigned) == len(party_ids):
                print("\nAll parties assigned:")
                for pid in party_ids:
                    q = queues[pid]
                    print(f"  {pid} → server={q.get('server_id')}  version={q.get('version')}")
                return True

            time.sleep(POLL_INTERVAL)

    # Print final state on timeout
    with httpx.Client(timeout=10.0) as client:
        print("\nTimeout — final state:")
        for pid in party_ids:
            try:
                q = get_queue(client, gateway, pid)
                print(f"  {pid}: status={q.get('status')}  server={q.get('server_id')}")
            except RuntimeError as e:
                print(f"  {pid}: {e}")

    return False


# ---------------------------------------------------------------------------
# Phase 2 verification — WebSocket notifications
# ---------------------------------------------------------------------------

async def _listen_for_matchmake(ws_url: str, token: str, player: str, timeout: float) -> dict | None:
    """Connect to the subscriber WebSocket and wait for a Matchmake notification."""
    try:
        async with websockets.connect(
            ws_url,
            additional_headers={"Cookie": f"session={token}"},
        ) as ws:
            deadline = asyncio.get_event_loop().time() + timeout
            while asyncio.get_event_loop().time() < deadline:
                remaining = deadline - asyncio.get_event_loop().time()
                try:
                    raw = await asyncio.wait_for(ws.recv(), timeout=min(remaining, 2.0))
                    data = json.loads(raw)
                    if data.get("type") == "Matchmake":
                        print(f"  [{player}] received Matchmake: {data['payload']}")
                        return data["payload"]
                    # other notification types — keep waiting
                except asyncio.TimeoutError:
                    continue
    except Exception as e:
        print(f"  [{player}] WebSocket error: {e}")
    print(f"  [{player}] timed out waiting for Matchmake")
    return None


async def _run_ws_phase(subscriber_url: str, tokens: list[tuple[str, str]], timeout: float) -> list[dict | None]:
    ws_url = subscriber_url.replace("http://", "ws://").replace("https://", "wss://") + "/api/v1/ws"
    tasks = [_listen_for_matchmake(ws_url, token, username, timeout) for username, token in tokens]
    return await asyncio.gather(*tasks)


def run_ws_notification_test(gateway: str, gatewaypriv: str, subscriber: str, mongo_url: str) -> bool:
    print("\n── Phase 2: WebSocket notification test ──")

    # Setup player accounts
    print("\nCreating player accounts …")
    tokens: list[tuple[str, str]] = []  # (username, jwt)
    with httpx.Client(timeout=30.0) as client:
        for p in WS_PLAYERS:
            ensure_account(client, gateway, p["username"], p["email"], p["password"])
            token = login(client, gateway, p["username"], p["password"])
            user_id = decode_user_id(token)
            tokens.append((p["username"], token))
            print(f"  logged in: {p['username']} → user_id={user_id}")

        player_ids = [decode_user_id(t) for _, t in tokens]

        print("\nRegistering WS test server …")
        register_server(client, gatewaypriv, WS_SERVER)

        # Start WebSocket listeners BEFORE queuing so we don't miss the notification
        print(f"\nConnecting {len(WS_PLAYERS)} WebSocket clients …")
        ws_future = asyncio.ensure_future if False else None  # placeholder

        async def run():
            # Start all WS listeners concurrently with a small head start before queuing
            ws_task = asyncio.create_task(
                _run_ws_phase(subscriber, tokens, WS_TIMEOUT)
            )
            # Give connections a moment to establish before queuing
            await asyncio.sleep(1.0)

            # Queue the party using real user_ids
            queue_party(client, gateway, {
                "party_id":   WS_PARTY_ID,
                "player_ids": player_ids,
            })

            return await ws_task

        results = asyncio.run(run())

    # Verify all players got notified
    ok = True
    print("\nNotification results:")
    for (username, _), result in zip(tokens, results):
        if result is None:
            print(f"  FAIL {username}: no Matchmake received")
            ok = False
        else:
            print(f"  PASS {username}: server_id={result.get('server_id')}  "
                  f"ip={result.get('server_ip')}  port={result.get('server_port')}")

    # Cleanup WS test data from MongoDB
    mdb = pymongo.MongoClient(mongo_url)
    mdb["mm"]["gameservers"].delete_one({"_id": WS_SERVER["server_id"]})
    mdb["mm"]["pending_parties"].delete_one({"_id": WS_PARTY_ID})
    mdb.close()

    return ok


# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------

def cleanup(mongo_url: str) -> None:
    client = pymongo.MongoClient(mongo_url)
    server_ids = [gs["server_id"] for gs in GAME_SERVERS]
    party_ids  = [p["party_id"]   for p in PARTIES]
    gs_result    = client["mm"]["gameservers"].delete_many({"_id": {"$in": server_ids}})
    party_result = client["mm"]["pending_parties"].delete_many({"_id": {"$in": party_ids}})
    print(f"Cleanup: removed {gs_result.deleted_count} game servers, {party_result.deleted_count} parties")
    client.close()


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main() -> None:
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--gateway", "-g", default=GATEWAY_DEFAULT,
                        help=f"Gateway base URL (default: {GATEWAY_DEFAULT})")
    parser.add_argument("--gatewaypriv", "-p", default=GATEWAYPRIV_DEFAULT,
                        help=f"Private gateway base URL (default: {GATEWAYPRIV_DEFAULT})")
    parser.add_argument("--subscriber", "-s", default=SUBSCRIBER_DEFAULT,
                        help=f"Subscriber base URL (default: {SUBSCRIBER_DEFAULT})")
    parser.add_argument("--mongo", "-m", default=MONGO_DEFAULT,
                        help=f"MongoDB URL (default: {MONGO_DEFAULT})")
    parser.add_argument("--no-cleanup", action="store_true",
                        help="Skip MongoDB cleanup before and after the test")
    parser.add_argument("--ws", action="store_true",
                        help="Also run Phase 2: WebSocket notification test")
    args = parser.parse_args()

    # ── Phase 1 ──────────────────────────────────────────────────────────────
    print("── Phase 1: Assignment test ──")

    if not args.no_cleanup:
        print("Pre-test cleanup …")
        cleanup(args.mongo)

    with httpx.Client(timeout=30.0) as client:
        print("\nRegistering game servers …")
        for gs in GAME_SERVERS:
            register_server(client, args.gatewaypriv, gs)

        print("\nQueuing parties …")
        party_ids = []
        for party in PARTIES:
            queue_id = queue_party(client, args.gateway, party)
            party_ids.append(queue_id)

    phase1_ok = wait_for_assignments(args.gateway, party_ids)

    if not args.no_cleanup:
        cleanup(args.mongo)

    # ── Phase 2 ──────────────────────────────────────────────────────────────
    phase2_ok = True
    if args.ws:
        phase2_ok = run_ws_notification_test(args.gateway, args.gatewaypriv, args.subscriber, args.mongo)

    # ── Result ───────────────────────────────────────────────────────────────
    print()
    if phase1_ok:
        print("Phase 1 PASS")
    else:
        print("Phase 1 FAIL — solver did not assign all parties within the timeout")

    if args.ws:
        if phase2_ok:
            print("Phase 2 PASS")
        else:
            print("Phase 2 FAIL — not all players received a Matchmake notification")

    if not (phase1_ok and phase2_ok):
        raise SystemExit(1)
    print("\nPASS")


if __name__ == "__main__":
    main()

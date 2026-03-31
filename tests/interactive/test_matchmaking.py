"""
Integration test for the matchmaking pipeline.

Seeds game servers and parties via the gateway HTTP API, then polls
MongoDB until the mmsolver assigns all parties, or times out.

Usage:
    python test_matchmaking.py
    python test_matchmaking.py --gateway http://localhost:9001 --mongo mongodb://root:root@localhost:27017
"""
import argparse
import time
import uuid

import httpx
import pymongo

GATEWAY_DEFAULT = "http://localhost:9001"
MONGO_DEFAULT   = "mongodb://root:root@localhost:27017"
POLL_INTERVAL   = 1.0   # seconds between MongoDB checks
POLL_TIMEOUT    = 30.0  # seconds before giving up

GAME_SERVERS = [
    {"server_id": "gs-test-1", "ip_address": "10.0.0.1", "port": 7001, "capacity": 4},
    {"server_id": "gs-test-2", "ip_address": "10.0.0.2", "port": 7002, "capacity": 6},
]

PARTIES = [
    {"party_id": "party-test-alpha", "player_ids": [str(uuid.uuid4()), str(uuid.uuid4())]},
    {"party_id": "party-test-beta",  "player_ids": [str(uuid.uuid4()), str(uuid.uuid4()), str(uuid.uuid4())]},
    {"party_id": "party-test-gamma", "player_ids": [str(uuid.uuid4())]},
    {"party_id": "party-test-delta", "player_ids": [str(uuid.uuid4()), str(uuid.uuid4()), str(uuid.uuid4())]},
]


# ---------------------------------------------------------------------------
# Seed helpers
# ---------------------------------------------------------------------------

def register_server(client: httpx.Client, gateway: str, gs: dict) -> None:
    resp = client.post(f"{gateway}/api/v1/mm/register/gameserver", json={
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
# Verification
# ---------------------------------------------------------------------------

def wait_for_assignments(mongo_url: str, party_ids: list) -> bool:
    client = pymongo.MongoClient(mongo_url)
    col = client["mm"]["pending_parties"]

    print(f"\nPolling MongoDB for assignments (timeout={POLL_TIMEOUT}s) …")
    deadline = time.monotonic() + POLL_TIMEOUT
    while time.monotonic() < deadline:
        docs = {d["_id"]: d for d in col.find({"_id": {"$in": party_ids}})}
        assigned = [pid for pid in party_ids if docs.get(pid, {}).get("status") == "assigned"]
        pending  = [pid for pid in party_ids if docs.get(pid, {}).get("status") == "pending"]
        missing  = [pid for pid in party_ids if pid not in docs]

        print(f"  assigned={len(assigned)}/{len(party_ids)}  pending={len(pending)}  missing={len(missing)}")

        if len(assigned) == len(party_ids):
            print("\nAll parties assigned:")
            for pid in party_ids:
                doc = docs[pid]
                print(f"  {pid} → server={doc.get('server_id')}  version={doc.get('version')}")
            client.close()
            return True

        time.sleep(POLL_INTERVAL)

    docs = {d["_id"]: d for d in col.find({"_id": {"$in": party_ids}})}
    print("\nTimeout — final state:")
    for pid in party_ids:
        doc = docs.get(pid, {})
        print(f"  {pid}: status={doc.get('status')}  server={doc.get('server_id')}")

    client.close()
    return False


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
    parser.add_argument("--mongo", "-m", default=MONGO_DEFAULT,
                        help=f"MongoDB URL (default: {MONGO_DEFAULT})")
    parser.add_argument("--no-cleanup", action="store_true",
                        help="Skip MongoDB cleanup before and after the test")
    args = parser.parse_args()

    if not args.no_cleanup:
        print("Pre-test cleanup …")
        cleanup(args.mongo)

    with httpx.Client(timeout=30.0) as client:
        print("\nRegistering game servers …")
        for gs in GAME_SERVERS:
            register_server(client, args.gateway, gs)

        print("\nQueuing parties …")
        party_ids = []
        for party in PARTIES:
            queue_id = queue_party(client, args.gateway, party)
            party_ids.append(queue_id)

    ok = wait_for_assignments(args.mongo, party_ids)

    if not args.no_cleanup:
        cleanup(args.mongo)

    if ok:
        print("\nPASS")
    else:
        print("\nFAIL — solver did not assign all parties within the timeout")
        raise SystemExit(1)


if __name__ == "__main__":
    main()

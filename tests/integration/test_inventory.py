"""
Integration tests for the inventory service via the portal WebSocket RPC.

Flow tested:
  - GetInventory: error for unknown player
  - AddItem: auto-placement, stacking, stack overflow, idempotency, full inventory
  - AddItemToSlot: explicit slot targeting, conflict, idempotency

create_inventory uses gatewaypriv HTTP (internal op, no WS endpoint).
All other inventory operations go through the portal WebSocket.

Requires: portal + gatewaypriv + inventory service + Postgres + RabbitMQ.

Usage:
    pytest test_inventory.py
"""
import uuid
import pytest
import httpx
import websockets
from ulid import ULID
import portal_pb2
from wsrpc import WsRpcClient


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def new_order_id() -> str:
    return str(ULID())


def new_player_id(tag: str) -> str:
    return f"inv-test-{tag}-{uuid.uuid4().hex[:8]}"


async def ws_get_inventory(rpc: WsRpcClient, player_id: str) -> tuple[int, portal_pb2.InventoryGetResponse | str]:
    status, payload = await rpc.call(portal_pb2.GameMsg.INVENTORY_GET_REQUEST,
                                     portal_pb2.InventoryGetRequest(player_id=player_id))
    if status != 0:
        return status, payload.decode()
    res = portal_pb2.InventoryGetResponse()
    res.ParseFromString(payload)
    return status, res


async def ws_add_item(rpc: WsRpcClient, player_id: str, item_id: str,
                      amount: int, max_stack: int, order_id: str) -> tuple[int, portal_pb2.InventoryGetResponse | str]:
    status, payload = await rpc.call(portal_pb2.GameMsg.INVENTORY_ADD_ITEM,
                                     portal_pb2.InventoryAddItemRequest(
                                         player_id=player_id, item_id=item_id,
                                         amount=amount, max_stack=max_stack, order_id=order_id))
    if status != 0:
        return status, payload.decode()
    res = portal_pb2.InventoryGetResponse()
    res.ParseFromString(payload)
    return status, res


async def ws_add_item_to_slot(rpc: WsRpcClient, player_id: str, item_id: str, slot_id: int,
                              amount: int, max_stack: int, order_id: str) -> tuple[int, portal_pb2.InventoryGetResponse | str]:
    status, payload = await rpc.call(portal_pb2.GameMsg.INVENTORY_ADD_ITEM_TO_SLOT,
                                     portal_pb2.InventoryAddItemToSlotRequest(
                                         player_id=player_id, item_id=item_id, slot_id=slot_id,
                                         amount=amount, max_stack=max_stack, order_id=order_id))
    if status != 0:
        return status, payload.decode()
    res = portal_pb2.InventoryGetResponse()
    res.ParseFromString(payload)
    return status, res


# ---------------------------------------------------------------------------
# HTTP helper: create_inventory (internal op, no WS endpoint)
# ---------------------------------------------------------------------------

def http_create_inventory(gatewaypriv: str, player_id: str) -> None:
    with httpx.Client(timeout=10.0) as client:
        resp = client.post(f"{gatewaypriv}/api/v1/inventory/create", json={"player_id": player_id})
    assert resp.status_code == 200, f"create_inventory failed ({resp.status_code}): {resp.text}"


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def player(gatewaypriv):
    """Yields a unique player_id and creates the inventory via HTTP."""
    player_id = new_player_id("test")
    http_create_inventory(gatewaypriv, player_id)
    yield player_id


# ---------------------------------------------------------------------------
# Tests: GetInventory
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_get_inventory_not_found(portal_ws_url):
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        status, _ = await ws_get_inventory(rpc, f"no-such-player-{uuid.uuid4().hex}")
    assert status != 0


# ---------------------------------------------------------------------------
# Tests: AddItem
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_add_item_without_inventory_returns_error(portal_ws_url):
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        status, _ = await ws_add_item(rpc, new_player_id("noninv"), "sword", 1, 10, new_order_id())
    assert status != 0


@pytest.mark.asyncio
async def test_add_item_places_item(portal_ws_url, player):
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        status, inv = await ws_add_item(rpc, player, "sword", 1, 10, new_order_id())
    assert status == 0
    assert sum(i.amount for i in inv.inventory if i.item_id == "sword") == 1


@pytest.mark.asyncio
async def test_add_item_stacks_same_item(portal_ws_url, player):
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        await ws_add_item(rpc, player, "arrow", 10, 99, new_order_id())
        status, inv = await ws_add_item(rpc, player, "arrow", 15, 99, new_order_id())
    assert status == 0
    assert sum(i.amount for i in inv.inventory if i.item_id == "arrow") == 25
    assert sum(1 for i in inv.inventory if i.item_id == "arrow") == 1


@pytest.mark.asyncio
async def test_add_item_spills_to_new_slot_when_stack_full(portal_ws_url, player):
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        await ws_add_item(rpc, player, "potion", 5, 5, new_order_id())
        status, inv = await ws_add_item(rpc, player, "potion", 5, 5, new_order_id())
    assert status == 0
    assert sum(i.amount for i in inv.inventory if i.item_id == "potion") == 10
    assert sum(1 for i in inv.inventory if i.item_id == "potion") == 2


@pytest.mark.asyncio
async def test_add_item_multiple_types(portal_ws_url, player):
    items = [("sword", 1), ("shield", 1), ("helmet", 1)]
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        for item_id, amount in items:
            status, inv = await ws_add_item(rpc, player, item_id, amount, 1, new_order_id())
    assert status == 0
    for item_id, expected in items:
        assert sum(i.amount for i in inv.inventory if i.item_id == item_id) == expected


@pytest.mark.asyncio
async def test_add_item_idempotent(portal_ws_url, player):
    order_id = new_order_id()
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        status1, inv1 = await ws_add_item(rpc, player, "gem", 50, 100, order_id)
        status2, inv2 = await ws_add_item(rpc, player, "gem", 50, 100, order_id)
    assert status1 == 0 and status2 == 0
    assert sum(i.amount for i in inv1.inventory if i.item_id == "gem") == 50
    assert sum(i.amount for i in inv2.inventory if i.item_id == "gem") == 50


@pytest.mark.asyncio
async def test_add_item_inventory_full_returns_error(portal_ws_url, player):
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        for i in range(20):
            await ws_add_item(rpc, player, f"item_{i}", 1, 1, new_order_id())
        status, _ = await ws_add_item(rpc, player, "overflow_item", 1, 1, new_order_id())
    assert status != 0


# ---------------------------------------------------------------------------
# Tests: AddItemToSlot
# ---------------------------------------------------------------------------

@pytest.mark.asyncio
async def test_add_item_to_slot_places_in_correct_slot(portal_ws_url, player):
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        status, inv = await ws_add_item_to_slot(rpc, player, "dagger", 5, 3, 10, new_order_id())
    assert status == 0
    assert sum(i.amount for i in inv.inventory if i.item_id == "dagger") == 3


@pytest.mark.asyncio
async def test_add_item_to_slot_stacks_same_item(portal_ws_url, player):
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        await ws_add_item_to_slot(rpc, player, "arrow", 5, 3, 20, new_order_id())
        status, inv = await ws_add_item_to_slot(rpc, player, "arrow", 5, 3, 20, new_order_id())
    assert status == 0
    assert sum(i.amount for i in inv.inventory if i.item_id == "arrow") == 6


@pytest.mark.asyncio
async def test_add_item_to_slot_conflict_wrong_item(portal_ws_url, player):
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        s1, _ = await ws_add_item_to_slot(rpc, player, "sword", 2, 1, 10, new_order_id())
        assert s1 == 0
        status, _ = await ws_add_item_to_slot(rpc, player, "shield", 2, 1, 10, new_order_id())
    assert status != 0


@pytest.mark.asyncio
async def test_add_item_to_slot_conflict_stack_overflow(portal_ws_url, player):
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        s1, _ = await ws_add_item_to_slot(rpc, player, "arrow", 3, 5, 5, new_order_id())
        assert s1 == 0
        status, _ = await ws_add_item_to_slot(rpc, player, "arrow", 3, 5, 5, new_order_id())
    assert status != 0


@pytest.mark.asyncio
async def test_add_item_to_slot_idempotent(portal_ws_url, player):
    order_id = new_order_id()
    async with websockets.connect(portal_ws_url) as ws:
        rpc = WsRpcClient(ws)
        s1, inv1 = await ws_add_item_to_slot(rpc, player, "rune", 7, 4, 10, order_id)
        s2, inv2 = await ws_add_item_to_slot(rpc, player, "rune", 7, 4, 10, order_id)
    assert s1 == 0 and s2 == 0
    assert sum(i.amount for i in inv1.inventory if i.item_id == "rune") == 4
    assert sum(i.amount for i in inv2.inventory if i.item_id == "rune") == 4

import asyncio
import struct
import websockets
import portal_pb2


def build_packet(msg_type: int, seq_id: int, payload: bytes) -> bytes:
    header = struct.pack(">HI", msg_type, seq_id)
    return header + payload


def parse_packet(data: bytes) -> tuple[int, int, int, bytes]:
    res_type, res_seq_id = struct.unpack(">HI", data[:6])
    status = data[6]
    return res_type, res_seq_id, status, data[7:]


async def send_trade(websocket, seq_id: int, item_id: int):
    req = portal_pb2.TradeRequest()
    req.item_id = item_id
    packet = build_packet(portal_pb2.GameMsg.TRADE_REQUEST, seq_id, req.SerializeToString())

    print(f"[seq={seq_id}] Sending TradeRequest item_id={item_id}")
    await websocket.send(packet)

    data = await websocket.recv()
    res_type, res_seq_id, status, res_payload = parse_packet(data)

    assert res_seq_id == seq_id, f"seq_id mismatch: sent {seq_id}, got {res_seq_id}"

    if status != 0:
        error_msg = res_payload.decode("utf-8")
        print(f"[seq={res_seq_id}] Error — {error_msg!r}")
        return None

    res = portal_pb2.TradeResponse()
    res.ParseFromString(res_payload)
    print(f"[seq={res_seq_id}] Response — success={res.success} message={res.message!r}")
    return res


async def test_long_lived_websocket():
    uri = "ws://localhost:9003/api/v1/portal/ws"

    trades = [
        (1, 100),
        (2, 200),
        (3, 300),
        (4, 404),   # unknown item — expect failure
        (5, 500),
    ]

    async with websockets.connect(uri) as websocket:
        print(f"Connected to {uri}")
        for seq_id, item_id in trades:
            await send_trade(websocket, seq_id, item_id)

        print(f"\nAll {len(trades)} trades completed on a single connection.")


if __name__ == "__main__":
    asyncio.run(test_long_lived_websocket())

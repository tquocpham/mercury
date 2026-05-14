"""
Shared WebSocket RPC client for integration tests.

Wraps binary packet framing:
    Header: [Type(2) | SeqID(4) | Status(1) | Payload(N)]
    Status 0 = OK, non-zero = error (payload is a UTF-8 error string).
"""
import struct


class WsRpcClient:
    def __init__(self, ws):
        self._ws = ws
        self._seq = 0

    def _next_seq(self) -> int:
        self._seq += 1
        return self._seq

    async def call(self, msg_type: int, proto_msg) -> tuple[int, bytes]:
        """Send a proto message and return (status, payload)."""
        seq_id = self._next_seq()
        packet = struct.pack(">HI", msg_type, seq_id) + proto_msg.SerializeToString()
        await self._ws.send(packet)

        data = await self._ws.recv()
        res_type, res_seq_id = struct.unpack(">HI", data[:6])
        status = data[6]
        payload = data[7:]

        assert res_seq_id == seq_id, f"seq_id mismatch: sent {seq_id}, got {res_seq_id}"
        return status, payload

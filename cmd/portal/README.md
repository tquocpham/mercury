# Portal

Portal is the game server-facing WebSocket service. Game clients connect to Portal over a persistent WebSocket and call backend services (inventory, trade, etc.) using a binary RPC protocol backed by Protocol Buffers.

## Protocol

All messages share a fixed 7-byte header:

```
[Type: 2 bytes | SeqID: 4 bytes | Status: 1 byte] [Payload: N bytes]
```

| Field  | Size | Description |
|--------|------|-------------|
| Type   | 2    | Message type from the `GameMsg` enum |
| SeqID  | 4    | Client-chosen sequence ID; echoed in the response for correlation |
| Status | 1    | `0` = OK, non-zero = error |
| Payload| N    | Serialized protobuf message (or UTF-8 error string on error) |

All integers are **big-endian**.

## Endpoint

```
GET /api/v1/portal/ws
```

Upgrade to WebSocket. The connection stays open for the duration of the game session.

## Messages

Defined in [portal.proto](portal.proto). Run `make proto` from the repo root to regenerate bindings.

| GameMsg                  | Value | Request                        | Response               |
|--------------------------|-------|--------------------------------|------------------------|
| `TRADE_REQUEST`          | 12    | `TradeRequest`                 | `TradeResponse`        |
| `INVENTORY_GET_REQUEST`  | 21    | `InventoryGetRequest`          | `InventoryGetResponse` |
| `INVENTORY_ADD_ITEM`     | 23    | `InventoryAddItemRequest`      | `InventoryGetResponse` |
| `INVENTORY_ADD_ITEM_TO_SLOT` | 24 | `InventoryAddItemToSlotRequest` | `InventoryGetResponse` |

## Adding a new message type

1. Add the enum value to `GameMsg` in `portal.proto`
2. Define the request/response messages in `portal.proto`
3. Run `make proto` to regenerate bindings
4. Add a handler function in `lib/rpchandlers.go`
5. Register it in `main.go` with `rpc.Register(...)`

## Configuration

| Env var      | Default | Description        |
|--------------|---------|--------------------|
| `WEB_PORT`   | `80`    | HTTP/WebSocket port |
| `AMQP_URL`   | —       | RabbitMQ connection string |
| `LOG_LEVEL`  | `info`  | Logging level |
| `STATSD_ADDR`| —       | StatsD address for metrics |

## Running locally

```bash
cd cmd/portal && go run .
```

Portal listens on port `9003` in the local dev config.

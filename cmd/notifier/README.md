# Notifier Service Design

## Abstract

A generic real-time notification service. Any internal service (gateway, matchmaking,
game server) publishes an event to the notifier. The notifier forwards it over WebSocket
to all subscribed clients. Built on Redis pub/sub for fan-out across multiple notifier
instances.

Not chat-specific. Not game-specific. A general-purpose push channel.

## Technical Requirements

- Redis pub/sub to push events to connected clients
- Extensible event types — the list is open-ended:
  - Chat messages
  - Pop-up notifications
  - In-game event notifications
  - In-game mail
  - Match state updates
  - Lobby changes
  - Leaderboard updates
  - Any future event type

---

## Channel Model

Every notification belongs to a **channel**. A channel is a string with the form:

```
{namespace}:{id}
```

Examples:

| Channel | Used for |
|---|---|
| `chat:convo-abc` | Messages in a conversation |
| `game:match-xyz` | Game state updates for a match |
| `player:user-123` | Personal notifications (achievements, friend requests, mail) |
| `lobby:lobby-456` | Lobby membership / ready state |
| `leaderboard:global` | Global leaderboard changes |

Namespaces are not registered or validated by the notifier — they are a naming convention
enforced by the publishing service. The notifier treats all channels identically.

---

## Message Envelope

Every message delivered to a client has this shape, regardless of namespace:

```json
{
  "channel": "game:match-xyz",
  "type":    "state_update",
  "seq":     42,
  "payload": { }
}
```

| Field | Description |
|---|---|
| `channel` | The channel this notification came from |
| `type` | Event type — defined by the publisher, interpreted by the client |
| `seq` | Monotonically increasing integer per channel (stored in Redis). Used by clients to detect missed messages. |
| `payload` | Arbitrary JSON — whatever the publisher sends |

The notifier does not inspect or validate `type` or `payload`. Publishers own their schema.

---

## Client Protocol (WebSocket)

```
GET /api/v1/ws
```

**Step 1 — Connect and Subscribe**

Client opens a WebSocket connection. Sends an auth token in the first message (see Authorization).
Client sends one JSON message declaring which channels it wants:

```json
{
  "token": "eyJ...",
  "channels": ["chat:convo-abc", "game:match-xyz", "player:user-123"]
}
```

Multiple channels over one connection. Client can be in a lobby, a game, and a chat
simultaneously.

**Step 2 — Receive**

Server streams notifications as they arrive. Each message is the envelope format above.
Client stays connected until it disconnects.

---

## Publisher API (HTTP)

Internal services publish to the notifier via **HTTP POST**.

```
POST /publish
Content-Type: application/json

{
  "channel": "game:match-xyz",
  "type":    "state_update",
  "payload": "\{\"turn\":\"player-1\",\"board\": ...\}",
}
```

Response:

```json
{ "notified": 3 }
```

`notified` is the number of subscribers Redis delivered to across all notifier instances.

The notifier:
1. Increments `INCR notifier:{channel}:seq` in Redis to get the next seq number
2. Assembles the envelope with seq
3. Publishes the envelope JSON to Redis channel `{channel}`
4. Returns `notified`

---

## Architecture

```
internal service
  │
  └── POST /publish ──► notifier ──► INCR seq in Redis
                                  └► PUBLISH {channel} envelope ──► Redis pub/sub
                                                                          │
                                              ┌─────────────────────────┤
                                              │                          │
                                        notifier-1                notifier-2
                                     (subscribed to              (subscribed to
                                      {channel})                  {channel})
                                              │                          │
                                        ws client A             ws client B
                                   (in game:match-xyz)       (in game:match-xyz)
```

Each notifier instance subscribes to Redis channels dynamically as clients connect.
When a Redis message arrives, the instance fans out to all locally-connected clients
subscribed to that channel.

---

## Authorization

Each namespace defines its own authorization rules. The notifier enforces them via a
pluggable **channel authorizer**.

On subscribe, before the Redis subscription is opened, the notifier checks whether the
client is allowed to subscribe to each requested channel.

```
client connects with JWT token
  │
  ├── parse + validate JWT → get user_id
  │
  └── for each requested channel:
        authorizer.CanSubscribe(user_id, channel) → bool
```

Authorizer rules per namespace:

| Namespace | Rule |
|---|---|
| `chat:{convo_id}` | user is a participant in the conversation |
| `game:{match_id}` | user is a player in the match |
| `player:{user_id}` | user_id matches the authenticated user |
| `lobby:{lobby_id}` | user is a member of the lobby |
| `leaderboard:*` | public, always allowed |

If authorization fails for a channel, the notifier sends an error frame for that channel
and continues subscribing to the rest. It does not close the connection.

---

## Reliability

The notifier is a **best-effort delivery layer**. It does not guarantee delivery.

| Failure | Behavior |
|---|---|
| Redis blip | Messages published during outage are lost. Client detects gap via seq and fetches from source. |
| Notifier instance crash | Client WebSocket drops. Client reconnects, re-subscribes, fetches missed data from source. |
| Client disconnect | No buffering. Client re-fetches on reconnect. |

Source-of-truth services (Cassandra, Postgres) always have the full history. The
notifier is a shortcut for real-time delivery, not a database.

---

## Open Questions

- **Auth transport**: JWT in query param (`/ws?token=...`) vs first WebSocket message?
  Query param is simpler but token appears in server logs. First message keeps token
  out of logs.

- **Mid-session subscribe/unsubscribe**: Should clients be able to add/remove channels
  after the initial subscribe? (e.g. player joins a new lobby mid-session). Current
  design: one subscribe message at connect. Can extend later.

- **Authorizer interface**: Should authorization be in-process (fast, no network) or
  call out to owning services (accurate, slower)? Hybrid option: JWT claims for
  identity, in-process rules per namespace using claims.

- **`/send` → `/publish` migration**: The existing WebSocket-based `/send` endpoint
  and the `pkg/clients/notifier` client need to be replaced with HTTP POST.

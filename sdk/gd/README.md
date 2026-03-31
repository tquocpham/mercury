# Mercury Godot 4 SDK

Two autoload singletons for integrating with the Mercury matchmaking backend.

---

## MercuryClient.gd

Player client SDK.

### Setup

1. Copy `MercuryClient.gd` into your Godot project (e.g. `res://addons/mercury/`).
2. Add it as an autoload singleton:
   **Project > Project Settings > Autoload > Add** `"MercuryClient"` pointing to this file.
3. Set the URLs to match your deployment before calling `login()`:
   ```gdscript
   MercuryClient.gateway_url    = "http://your-server:9001"
   MercuryClient.subscriber_url = "http://your-server:9004"
   ```

### Quick Start

```gdscript
MercuryClient.gateway_url    = "http://localhost:9001"
MercuryClient.subscriber_url = "http://localhost:9004"

MercuryClient.logged_in.connect(_on_logged_in)
MercuryClient.login_failed.connect(_on_login_failed)
MercuryClient.login("username", "password")

func _on_logged_in() -> void:
    MercuryClient.connect_notifications()
    MercuryClient.join_queue("party-001", ["player-uid-1", "player-uid-2"])

MercuryClient.matchmake_received.connect(_on_matchmake)
func _on_matchmake(server_id: String, server_ip: String, server_port: int) -> void:
    # Connect your game client to the game server here
    pass
```

### Signals

| Signal | Description |
|--------|-------------|
| `logged_in` | Login succeeded. Call `connect_notifications()` afterwards. |
| `login_failed(error: String)` | Login failed. |
| `notifications_connected` | WebSocket connection is open. |
| `notifications_disconnected` | WebSocket connection was closed. |
| `matchmake_received(server_id, server_ip, server_port)` | Solver assigned this party to a game server. |
| `message_received(payload: Dictionary)` | A chat/system message notification arrived. |
| `queue_joined(party_id: String)` | `join_queue()` succeeded. |
| `queue_failed(error: String)` | `join_queue()` failed. |
| `queue_status(data: Dictionary)` | Result from `get_queue()`. |

---

## MercuryGameServer.gd

Game server SDK. **This service should NOT be exposed to players.**

### Setup

1. Copy `MercuryGameServer.gd` into your Godot project (e.g. `res://addons/mercury/`).
2. Add it as an autoload singleton:
   **Project > Project Settings > Autoload > Add** `"MercuryGameServer"` pointing to this file.
3. Configure and register on startup:
   ```gdscript
   MercuryGameServer.gatewaypriv_url = "http://your-server:9002"
   MercuryGameServer.server_id  = "unique-server-id"
   MercuryGameServer.ip_address = "1.2.3.4"
   MercuryGameServer.port       = 7777
   MercuryGameServer.capacity   = 10
   ```

### Quick Start

```gdscript
MercuryGameServer.registered.connect(_on_registered)
MercuryGameServer.register_failed.connect(_on_register_failed)
MercuryGameServer.register()

func _on_registered() -> void:
    print("server is visible to the matchmaker")

# On shutdown:
MercuryGameServer.unregister()
```

### Signals

| Signal | Description |
|--------|-------------|
| `registered` | Server successfully registered with the matchmaker. |
| `register_failed(error: String)` | Registration failed. |
| `unregistered` | Server set to draining; matchmaker will stop sending new players. |
| `unregister_failed(error: String)` | Unregistration failed. |

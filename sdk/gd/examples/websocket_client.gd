# websocket_client.gd
# Example WebSocket client for Mercury notification server.
# Add as an autoload singleton (Project > Project Settings > Autoload).
#
# Usage:
#   Network.connect_to_server("wss://yourserver/ws/client", "your-jwt-token")
#   Network.message_received.connect(_on_message)
#   Network.send("PLAYER_ACTION", {"action": "jump"})

extends Node

signal message_received(type: String, payload: Variant)
signal connected
signal disconnected

var _socket := WebSocketPeer.new()
var _last_state := WebSocketPeer.STATE_CLOSED

func connect_to_server(url: String, token: String) -> void:
	_socket.handshake_headers = PackedStringArray(["Authorization: Bearer " + token])
	var err := _socket.connect_to_url(url)
	if err != OK:
		push_error("WebSocket: failed to connect to %s (err %d)" % [url, err])

func send(type: String, payload: Variant = null) -> void:
	if _socket.get_ready_state() != WebSocketPeer.STATE_OPEN:
		push_warning("WebSocket: tried to send while not connected")
		return
	var msg := JSON.stringify({"type": type, "payload": payload})
	_socket.send_text(msg)

func close() -> void:
	_socket.close(1000, "client closed")

func _process(_delta: float) -> void:
	_socket.poll()

	var state := _socket.get_ready_state()

	if state != _last_state:
		_on_state_changed(state)
		_last_state = state

	if state == WebSocketPeer.STATE_OPEN:
		while _socket.get_available_packet_count() > 0:
			_handle_packet(_socket.get_packet())

func _on_state_changed(state: int) -> void:
	match state:
		WebSocketPeer.STATE_OPEN:
			print("WebSocket: connected")
			connected.emit()
		WebSocketPeer.STATE_CLOSED:
			var code := _socket.get_close_code()
			var reason := _socket.get_close_reason()
			print("WebSocket: disconnected (code=%d reason=%s)" % [code, reason])
			disconnected.emit()

func _handle_packet(raw: PackedByteArray) -> void:
	var text := raw.get_string_from_utf8()
	var json = JSON.parse_string(text)
	if json == null:
		push_error("WebSocket: failed to parse packet: " + text)
		return
	if not json.has("type"):
		push_error("WebSocket: message missing 'type' field: " + text)
		return
	message_received.emit(json["type"], json.get("payload"))


# -----------------------------------------------------------------------------
# Example usage (attach to a Node in your scene):
# -----------------------------------------------------------------------------
#
# func _ready() -> void:
#     Network.message_received.connect(_on_message)
#     Network.connected.connect(_on_connected)
#     Network.connect_to_server("ws://localhost:8080/ws/client", $my_token)
#
# func _on_connected() -> void:
#     print("ready to receive notifications")
#
# func _on_message(type: String, payload: Variant) -> void:
#     match type:
#         "MESSAGE":
#             print("chat: ", payload["text"])
#         "TOAST":
#             print("toast: ", payload["message"])
#         "DISCONNECT":
#             Network.close()
#         _:
#             print("unhandled type: ", type, payload)

# MercuryClient.gd
# Mercury player client SDK for Godot 4.
#
# SETUP
#   1. Copy this file into your Godot project (e.g. res://addons/mercury/).
#   2. Add it as an autoload singleton:
#      Project > Project Settings > Autoload > Add "MercuryClient" pointing to this file.
#   3. Set the URLs to match your deployment before calling login():
#      MercuryClient.gateway_url    = "http://your-server:9001"
#      MercuryClient.subscriber_url = "http://your-server:9004"
#
# QUICK START
#   MercuryClient.gateway_url = "http://localhost:9001"
#   MercuryClient.subscriber_url = "http://localhost:9004"
#
#   MercuryClient.logged_in.connect(_on_logged_in)
#   MercuryClient.login_failed.connect(_on_login_failed)
#   MercuryClient.login("username", "password")
#
#   func _on_logged_in() -> void:
#       MercuryClient.connect_notifications()
#       MercuryClient.join_queue("party-001", ["player-uid-1", "player-uid-2"])
#
#   MercuryClient.matchmake_received.connect(_on_matchmake)
#   func _on_matchmake(server_id: String, server_ip: String, server_port: int) -> void:
#       # Connect your game client to the game server here
#       pass

extends Node

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

## Base URL of the public gateway (e.g. "http://localhost:9001")
var gateway_url: String = "http://localhost:9001"

## Base URL of the subscriber/notification service (e.g. "http://localhost:9004").
## The SDK converts this to a WebSocket URL automatically.
var subscriber_url: String = "http://localhost:9004"

# ---------------------------------------------------------------------------
# Signals
# ---------------------------------------------------------------------------

## Emitted when login succeeds. Call connect_notifications() afterwards.
signal logged_in

## Emitted when login fails.
signal login_failed(error: String)

## Emitted when the WebSocket notification connection is open.
signal notifications_connected

## Emitted when the WebSocket notification connection is closed.
signal notifications_disconnected

## Emitted when the solver assigns this party to a game server.
signal matchmake_received(server_id: String, server_ip: String, server_port: int)

## Emitted when a chat/system message notification arrives.
signal message_received(payload: Dictionary)

## Emitted when join_queue() succeeds.
signal queue_joined(party_id: String)

## Emitted when join_queue() fails.
signal queue_failed(error: String)

## Emitted when get_queue() returns a result.
signal queue_status(data: Dictionary)

# ---------------------------------------------------------------------------
# Private state
# ---------------------------------------------------------------------------

var _token: String = ""
var _socket := WebSocketPeer.new()
var _ws_last_state := WebSocketPeer.STATE_CLOSED

# ---------------------------------------------------------------------------
# Auth
# ---------------------------------------------------------------------------

## Log in with a username and password.
## On success, logged_in is emitted and _token is stored for subsequent calls.
func login(username: String, password: String) -> void:
	var body := JSON.stringify({
		"credentials": {"username": username, "password": password}
	})
	_request(
		gateway_url + "/api/v1/auth/login",
		HTTPClient.METHOD_POST,
		["Content-Type: application/json"],
		body,
		func(ok: bool, code: int, response: Dictionary) -> void:
			if not ok:
				login_failed.emit("HTTP %d" % code)
				return
			if not response.has("token"):
				login_failed.emit("malformed response")
				return
			_token = response["token"]
			logged_in.emit()
	)

# ---------------------------------------------------------------------------
# WebSocket notifications
# ---------------------------------------------------------------------------

## Open the persistent WebSocket connection to the notification service.
## Must be called after login() succeeds.
func connect_notifications() -> void:
	if _token.is_empty():
		push_error("MercuryClient: call login() before connect_notifications()")
		return
	var ws_url := _to_ws_url(subscriber_url) + "/api/v1/ws"
	_socket.handshake_headers = PackedStringArray(["Cookie: session=" + _token])
	var err := _socket.connect_to_url(ws_url)
	if err != OK:
		push_error("MercuryClient: WebSocket connect failed (err %d)" % err)

## Close the notification WebSocket.
func disconnect_notifications() -> void:
	_socket.close(1000, "client closed")

# ---------------------------------------------------------------------------
# Matchmaking
# ---------------------------------------------------------------------------

## Queue a party for matchmaking.
## party_id  — unique identifier for this party.
## player_ids — list of player user IDs in the party.
func join_queue(party_id: String, player_ids: Array) -> void:
	var body := JSON.stringify({"party_id": party_id, "player_ids": player_ids})
	_request(
		gateway_url + "/api/v1/mm/join/party",
		HTTPClient.METHOD_POST,
		["Content-Type: application/json", "Cookie: session=" + _token],
		body,
		func(ok: bool, code: int, response: Dictionary) -> void:
			if not ok:
				queue_failed.emit("HTTP %d" % code)
				return
			queue_joined.emit(response.get("party_id", party_id))
	)

## Poll the current status of a queued party.
## Result is emitted via queue_status.
func get_queue(party_id: String) -> void:
	_request(
		gateway_url + "/api/v1/mm/join/party/" + party_id,
		HTTPClient.METHOD_GET,
		["Cookie: session=" + _token],
		"",
		func(ok: bool, _code: int, response: Dictionary) -> void:
			if ok:
				queue_status.emit(response)
	)

# ---------------------------------------------------------------------------
# Godot lifecycle
# ---------------------------------------------------------------------------

func _process(_delta: float) -> void:
	_socket.poll()
	var state := _socket.get_ready_state()
	if state != _ws_last_state:
		_on_ws_state_changed(state)
		_ws_last_state = state
	if state == WebSocketPeer.STATE_OPEN:
		while _socket.get_available_packet_count() > 0:
			_handle_packet(_socket.get_packet())

# ---------------------------------------------------------------------------
# Private helpers
# ---------------------------------------------------------------------------

func _on_ws_state_changed(state: int) -> void:
	match state:
		WebSocketPeer.STATE_OPEN:
			notifications_connected.emit()
		WebSocketPeer.STATE_CLOSED:
			var code := _socket.get_close_code()
			var reason := _socket.get_close_reason()
			push_warning("MercuryClient: WebSocket closed (code=%d reason=%s)" % [code, reason])
			notifications_disconnected.emit()

func _handle_packet(raw: PackedByteArray) -> void:
	var text := raw.get_string_from_utf8()
	var json: Variant = JSON.parse_string(text)
	if json == null:
		push_error("MercuryClient: failed to parse notification: " + text)
		return
	var payload: Variant = json.get("payload", {})
	match json.get("type", ""):
		"Matchmake":
			matchmake_received.emit(
				payload.get("server_id", ""),
				payload.get("server_ip", ""),
				payload.get("server_port", 0)
			)
		"Message":
			message_received.emit(payload)
		"Disconnect":
			disconnect_notifications()
		var unknown:
			push_warning("MercuryClient: unhandled notification type: " + str(unknown))

func _request(
	url: String,
	method: int,
	headers: Array,
	body: String,
	callback: Callable
) -> void:
	var http := HTTPRequest.new()
	add_child(http)
	http.request_completed.connect(
		func(result: int, code: int, _headers: PackedStringArray, raw: PackedByteArray) -> void:
			http.queue_free()
			var ok := result == HTTPRequest.RESULT_SUCCESS and code == 200
			var json: Variant = JSON.parse_string(raw.get_string_from_utf8()) if ok else {}
			callback.call(ok, code, json if json != null else {})
	)
	var err := http.request(url, PackedStringArray(headers), method, body)
	if err != OK:
		http.queue_free()
		push_error("MercuryClient: request failed (err %d): %s" % [err, url])

func _to_ws_url(url: String) -> String:
	return url.replace("https://", "wss://").replace("http://", "ws://")

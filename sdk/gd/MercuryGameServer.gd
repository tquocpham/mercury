# MercuryGameServer.gd
# Mercury game server SDK for Godot 4.
#
# SETUP
#   1. Copy this file into your Godot project (e.g. res://addons/mercury/).
#   2. Add it as an autoload singleton:
#      Project > Project Settings > Autoload > Add "MercuryGameServer" pointing to this file.
#   3. Configure and register on startup:
#      MercuryGameServer.gatewaypriv_url = "http://your-server:9002"
#      MercuryGameServer.server_id  = "unique-server-id"
#      MercuryGameServer.ip_address = "1.2.3.4"
#      MercuryGameServer.port       = 7777
#      MercuryGameServer.capacity   = 10
#
# QUICK START
#   MercuryGameServer.registered.connect(_on_registered)
#   MercuryGameServer.register_failed.connect(_on_register_failed)
#   MercuryGameServer.register()
#
#   func _on_registered() -> void:
#       print("server is visible to the matchmaker")
#
#   # On shutdown:
#   MercuryGameServer.unregister()

extends Node

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

## Base URL of the private gateway (e.g. "http://localhost:9002").
## This service should NOT be exposed to players.
var gatewaypriv_url: String = "http://localhost:9002"

## Unique identifier for this server instance.
var server_id: String = ""

## Public IP address that players will connect to.
var ip_address: String = ""

## Port players will connect to.
var port: int = 7777

## Maximum number of players this server can host.
var capacity: int = 10

# ---------------------------------------------------------------------------
# Signals
# ---------------------------------------------------------------------------

## Emitted when the server is successfully registered with the matchmaker.
signal registered

## Emitted when registration fails.
signal register_failed(error: String)

## Emitted when the server is successfully unregistered (set to draining).
signal unregistered

## Emitted when unregistration fails.
signal unregister_failed(error: String)

# ---------------------------------------------------------------------------
# Private state
# ---------------------------------------------------------------------------

var _version: int = 0

# ---------------------------------------------------------------------------
# API
# ---------------------------------------------------------------------------

## Register this server with the matchmaker so it can receive players.
## Emits registered on success, register_failed on failure.
func register() -> void:
	if server_id.is_empty() or ip_address.is_empty():
		push_error("MercuryGameServer: server_id and ip_address must be set before calling register()")
		return
	var body := JSON.stringify({
		"server_id":  server_id,
		"ip_address": ip_address,
		"port":       port,
		"capacity":   capacity,
	})
	_request(
		gatewaypriv_url + "/api/v1/gs/register",
		HTTPClient.METHOD_POST,
		body,
		func(ok: bool, code: int, _response: Dictionary) -> void:
			if not ok:
				register_failed.emit("HTTP %d" % code)
				return
			_version = 0
			registered.emit()
	)

## Unregister this server, signalling to the matchmaker it should stop
## sending new players. Emits unregistered on success.
func unregister() -> void:
	var body := JSON.stringify({
		"server_id": server_id,
		"version":   _version,
	})
	_request(
		gatewaypriv_url + "/api/v1/gs/unregister",
		HTTPClient.METHOD_POST,
		body,
		func(ok: bool, code: int, _response: Dictionary) -> void:
			if not ok:
				unregister_failed.emit("HTTP %d" % code)
				return
			unregistered.emit()
	)

# ---------------------------------------------------------------------------
# Private helpers
# ---------------------------------------------------------------------------

func _request(
	url: String,
	method: int,
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
	var err := http.request(url, PackedStringArray(["Content-Type: application/json"]), method, body)
	if err != OK:
		http.queue_free()
		push_error("MercuryGameServer: request failed (err %d): %s" % [err, url])

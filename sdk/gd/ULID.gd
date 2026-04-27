
class_name ULID
extends Node

const ENCODING = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

static func generate() -> String:
	var time_ms = int(Time.get_unix_time_from_system() * 1000)
	return _encode_time(time_ms, 10) + _encode_random(16)

static func _encode_time(time: int, length: int) -> String:
	var res = ""
	for i in range(length):
		res = ENCODING[time % 32] + res
		time /= 32
	return res

static func _encode_random(length: int) -> String:
	var res = ""
	var crypto = Crypto.new()
	var bytes = crypto.get_random_bytes(length)
	for i in range(length):
		res += ENCODING[bytes[i] % 32]
	return res

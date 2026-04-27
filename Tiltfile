# Mercury Tiltfile
# Run: tilt up
# File watching is disabled on build resources — trigger builds manually
# from the Tilt UI or re-save this file to force a reload.

docker_compose("docker-compose.yml")

# ---------------------------------------------------------------------------
# Infrastructure (docker-compose managed, not rebuilt by Tilt)
# ---------------------------------------------------------------------------

infra = [
    "rabbitmq",
    "mongo",
    "redis",
    "influxdb",
    "telegraf",
    "grafana",
    "zookeeper",
    "kafka",
    "cassandra",
]

for svc in infra:
    dc_resource(svc, labels=["infra"])

# ---------------------------------------------------------------------------
# Go services — Tilt rebuilds these on source change
# ---------------------------------------------------------------------------

services = [
    "auth",
    "gateway",
    "gatewaypriv",
    "messages",
    "worker",
    "subscriber",
    "publisher",
    "mmservice",
    "mmsolver",
    "trade",
    "wallet",
    "courier",
]

for svc in services:
    dc_resource(svc, labels=["app"], trigger_mode=TRIGGER_MODE_MANUAL)

# ---------------------------------------------------------------------------
# Build resources — manual trigger, no file watching
# ---------------------------------------------------------------------------

go_services = [
    ("auth",        "cmd/auth"),
    ("gateway",     "cmd/gateway"),
    ("gatewaypriv", "cmd/gatewaypriv"),
    ("messages",    "cmd/messages"),
    ("worker",      "cmd/worker"),
    ("subscriber",  "cmd/subscriber"),
    ("publisher",   "cmd/publisher"),
    ("mmservice",   "cmd/mmservice"),
    ("mmsolver",    "cmd/mmsolver"),
    ("trade",       "cmd/trade"),
    ("wallet",      "cmd/wallet"),
    ("courier",     "cmd/courier"),
]

for svc, path in go_services:
    local_resource(
        "build:" + svc,
        cmd="cd " + path + " && go build -o ../../bin/" + svc + " .",
        deps=[],
        labels=["build"],
        resource_deps=[svc],
        auto_init=False,
        trigger_mode=TRIGGER_MODE_MANUAL,
    )

# ---------------------------------------------------------------------------
# Seeds — manual trigger only (click the button in the Tilt UI)
# ---------------------------------------------------------------------------

local_resource(
    "seed:users",
    cmd="python cicd/tilt/seed_users.py",
    resource_deps=["gateway", "auth", "mongo"],
    labels=["seed"],
    auto_init=False,
    trigger_mode=TRIGGER_MODE_MANUAL,
)

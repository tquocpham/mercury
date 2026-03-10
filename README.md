# Mercury

A Go microservices monorepo for a video games backend. Composing of a few different services:
Authentication, Notifications, Messaging, and Entitlements. More to come!
Messages are produced via an HTTP API, queued through Kafka for ordering guarantees, persisted to Cassandra, and delivered to connected clients in real time via WebSocket.

## Architecture

```
client
  │
  ├── POST /api/v1/auth ────────────► auth       ← JWT auth (MongoDB)
  │
  ├── GET  /api/v1/ws ──────────────► subscriber ← WebSocket push (Redis pub/sub)
  │                                       ▲
  │                                       │ POST /api/v1/send
  │                                   publisher   ← Internal publish endpoint
  │                                       ▲
  ▼                                       │
gateway          ← HTTP API          worker      ← Kafka consumer
  │   └── publishes to Kafka                └── writes to Cassandra
  │   └── reads via query client            └── notifies publisher
  │
  └── query      ← HTTP API → reads from Cassandra
```

**Infrastructure:** Kafka · Zookeeper · Cassandra · Redis · MongoDB · InfluxDB · Telegraf · Grafana

**Observability:** structured JSON logging (logrus), StatsD metrics (via Telegraf → InfluxDB → Grafana)

## Services

| Service | Description | Port | Network |
|---------|-------------|------|---------|
| `auth` | Authentication service — sign in, account creation, JWT issuance | `9005` | public |
| `gateway` | Public HTTP API — send and retrieve messages | `9001` | public |
| `query` | Internal read service — fetches messages from Cassandra | `9002` | internal |
| `worker` | Kafka consumer — persists messages, triggers notifications | — | internal |
| `subscriber` | Public WebSocket server — streams events to clients | `9004` | public |
| `publisher` | Internal HTTP endpoint — publishes events to Redis pub/sub | `9003` | internal |

### API (auth)

```
POST /api/v1/auth                        Sign in — returns a JWT
GET  /api/v1/auth/refresh                Refresh a JWT
POST /api/v1/account                     Create an account
POST /api/v1/account/activate/:accountid Activate an account
```

### API (gateway)

```
POST /api/v1/messages                                                    Send a message
GET  /api/v1/messages?conversation_id=&page_size=&next_token=            Paginated message history
GET  /api/v1/messages/refresh?conversation_id=&message_id=               Poll for new messages
```

### WebSocket (subscriber)

```
GET /api/v1/ws
```

Client connects, then sends a subscribe message as the first frame:

```json
{ "token": "eyJ...", "channels": ["chat:convo-abc", "player:user-123"] }
```

The server then streams JSON notification envelopes as events arrive. See [cmd/subscriber/README.md](cmd/subscriber/README.md) for the full channel model, message envelope format, and authorization design.

### Publish (publisher — internal only)

```
POST /api/v1/send
```

Used by internal services (e.g. worker) to push events to Redis pub/sub, which fans out to all connected subscriber instances:

```json
{ "channel": "chat:convo-abc", "payload": "{\"user\":\"alice\",\"message\":\"hi\"}" }
```

## Project Layout

```
mercury/
├── cmd/
│   ├── auth/        # Authentication service (JWT, MongoDB)
│   ├── gateway/     # Public HTTP API service
│   ├── subscriber/    # Public WebSocket notification service
│   │   └── README.md  # Notification system design doc
│   ├── publisher/   # Internal event publish service
│   ├── query/       # Internal read/query service
│   └── worker/      # Kafka consumer service
├── pkg/
│   ├── clients/
│   │   ├── publisher/ # HTTP client for publisher service
│   │   └── query/     # HTTP client for query service
│   ├── config/      # Viper-based config with env var + file support
│   ├── kmq/         # Kafka producer/consumer wrappers
│   ├── middleware/  # Echo middleware (logging, StatsD)
│   └── server/      # Shared graceful shutdown for Echo
├── tests/
│   └── interactive/ # Textual TUI chat client (Python)
├── docker/
│   └── telegraf/    # telegraf.conf
├── docker-compose.yml
├── docker-compose.override.yml   # local only — gitignored
├── Makefile
└── go.work
```

## Getting Started

### Prerequisites

- Go 1.25+
- Docker + Docker Compose

### Run locally

```bash
docker compose up
```

This starts all infrastructure (Kafka, Zookeeper, Cassandra, Redis, MongoDB, InfluxDB, Telegraf, Grafana) and all services.

### Build all services (binaries)

```bash
make          # build all → bin/
make gateway  # build one service
make clean    # remove bin/
make tidy     # go mod tidy across all modules
```

### Build Docker images

```bash
docker build -f cmd/auth/Dockerfile       -t mercury-auth .
docker build -f cmd/gateway/Dockerfile    -t mercury-gateway .
docker build -f cmd/query/Dockerfile      -t mercury-query .
docker build -f cmd/worker/Dockerfile     -t mercury-worker .
docker build -f cmd/subscriber/Dockerfile -t mercury-subscriber .
docker build -f cmd/publisher/Dockerfile  -t mercury-publisher .
```

## Configuration

All services read configuration from environment variables (uppercase). Viper's `AutomaticEnv` maps `WEB_PORT` → `web_port` config key. Config files (`config.yaml`, `config.local.yaml`) are merged if present for local development.

| Variable | Service(s) | Default | Description |
|----------|-----------|---------|-------------|
| `WEB_PORT` | all | `80` | HTTP listen port |
| `MONGO_ADDR` | auth | `mongodb://root:root@mongo:27017` | MongoDB connection string |
| `AWS_ACCESS_KEY` | auth, gateway | `test` | AWS access key (for SSM) |
| `AWS_SECRET_KEY` | auth, gateway | `test` | AWS secret key (for SSM) |
| `AWS_REGION` | auth, gateway | `us-west-1` | AWS region |
| `AWS_ENDPOINT` | auth, gateway | _(empty)_ | AWS endpoint override (e.g. LocalStack) |
| `KAFKA_BROKER` | gateway, worker | `kafka:29092` | Kafka broker address (internal listener) |
| `KAFKA_TOPIC` | gateway, worker | `messages` | Kafka topic |
| `KAFKA_GROUP_ID` | worker | `messages-consumer-group` | Kafka consumer group |
| `QUERY_HOST` | gateway | `http://query:9002` | Query service base URL |
| `PUBLISHER_ADDR` | worker | `http://publisher:9003` | Publisher service base URL |
| `CASSANDRA_HOST` | query, worker | `localhost` | Cassandra host |
| `REDIS_ADDR` | subscriber, publisher | `redis:6379` | Redis address |
| `REDIS_PW` | subscriber, publisher | _(empty)_ | Redis password |
| `LOG_LEVEL` | all | `info` | Log level |
| `ENVIRONMENT` | all | `local` | Environment label (added to logs) |
| `STATSD_ADDR` | all | `telegraf:8125` | StatsD UDP address |

## Interactive Test Client

A TUI chat client built with [Textual](https://textual.textualize.io/):

```bash
cd tests/interactive
pip install -r requirements.txt
python chatclient.py \
  --user alice \
  --password secret \
  --addr http://localhost:9001 \
  --auth-addr http://localhost:9005 \
  --ws-addr ws://localhost:9004 \
  --convoid my-chat
```

Messages are sent via the gateway HTTP API and received in real time via the subscriber WebSocket. A 30-second HTTP poll is used as a fallback.

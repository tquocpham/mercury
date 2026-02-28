# Mercury

A real-time chat backend built as a Go microservices monorepo. Messages are produced via an HTTP API, queued through Kafka for ordering guarantees, persisted to Cassandra, and served back through a dedicated query service.

## Architecture

```
client
  │
  ▼
gateway          ← HTTP API (port 8080)
  │   └── publishes to Kafka via worker client
  │   └── reads via query client (HTTP)
  │
  ├── worker     ← Kafka consumer → writes to Cassandra
  │
  └── query      ← HTTP API (port 9002) → reads from Cassandra
```

**Infrastructure:** Kafka · Cassandra · InfluxDB · Telegraf · Grafana

**Observability:** structured JSON logging (logrus), StatsD metrics (via Telegraf → InfluxDB → Grafana)

## Services

| Service | Description | Default Port |
|---------|-------------|--------------|
| `gateway` | Public-facing HTTP API — send and retrieve messages | `8080` |
| `query` | Internal read service — fetches messages from Cassandra | `9002` |
| `worker` | Kafka consumer — persists messages to Cassandra | — |

### API (gateway)

```
POST /api/v1/messages                        Send a message
GET  /api/v1/messages?conversation_id=&page_size=&next_token=   Paginated message history
GET  /api/v1/messages/refresh?conversation_id=&message_id=      Poll for new messages since message_id
```

## Project Layout

```
mercury/
├── cmd/
│   ├── gateway/     # HTTP gateway service
│   ├── query/       # Read/query service
│   └── worker/      # Kafka consumer service
├── pkg/
│   ├── clients/
│   │   ├── query/   # HTTP client for query service
│   │   └── worker/  # Kafka producer client
│   ├── config/      # Viper-based config with env var support
│   ├── instrumentation/ # StatsD metrics helpers
│   ├── kmq/         # Kafka producer/consumer wrappers
│   ├── middleware/  # Echo middleware (logging, StatsD, auth)
│   └── server/      # Shared graceful shutdown for Echo
├── tests/
│   └── interactive/ # Textual TUI chat client (Python)
├── docker-compose.yml
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

This starts Kafka, Cassandra, InfluxDB, Telegraf, Grafana, gateway, query, and worker.

### Build images individually

All builds use the repo root as context:

```bash
docker build -f cmd/gateway/Dockerfile -t mercury-gateway .
docker build -f cmd/query/Dockerfile   -t mercury-query .
docker build -f cmd/worker/Dockerfile  -t mercury-worker .
```

### Local development (without Docker)

The repo uses a [Go workspace](https://go.dev/ref/mod#workspaces) so `pkg/` is resolved locally across all services.

```bash
# Build a service
cd cmd/gateway && go build .

# After adding/changing pkg dependencies, tidy each service:
GOWORK=off go -C cmd/gateway mod tidy
GOWORK=off go -C cmd/query mod tidy
GOWORK=off go -C cmd/worker mod tidy
```

## Configuration

All services use environment variables (via [Viper's `AutomaticEnv`](https://github.com/spf13/viper)). Config files (`config.yaml`, `config.local.yaml`) are merged if present.

| Variable | Service | Default | Description |
|----------|---------|---------|-------------|
| `web_port` | gateway, query | `80` | HTTP listen port |
| `kafka_broker` | gateway, worker | `kafka:9092` | Kafka broker address |
| `kafka_topic` | gateway, worker | `messages` | Kafka topic |
| `query_host` | gateway | `http://query:9002` | Query service base URL |
| `cassandra_host` | query, worker | `localhost` | Cassandra host |
| `log_level` | all | `info` | Log level |
| `environment` | all | `local` | Environment label |
| `statsd_addr` | all | `telegraf:8125` | StatsD UDP address |

## Interactive Test Client

A TUI chat client built with [Textual](https://textual.textualize.io/):

```bash
cd tests/interactive
pip install -r requirements.txt
python chatclient.py --user alice --addr http://localhost:8080 --convoid my-chat
```

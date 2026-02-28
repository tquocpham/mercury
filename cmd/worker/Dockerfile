FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/worker/go.mod cmd/worker/go.sum cmd/worker/

WORKDIR /app/cmd/worker
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/worker/ cmd/worker/

WORKDIR /app/cmd/worker
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /worker .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /worker /worker

ENTRYPOINT ["/worker"]

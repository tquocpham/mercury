FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/inventory/go.mod cmd/inventory/go.sum cmd/inventory/

WORKDIR /app/cmd/inventory
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/inventory/ cmd/inventory/

WORKDIR /app/cmd/inventory
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /inventory .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /inventory /inventory

ENTRYPOINT ["/inventory"]

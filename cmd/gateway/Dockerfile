FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/gateway/go.mod cmd/gateway/go.sum cmd/gateway/

WORKDIR /app/cmd/gateway
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/gateway/ cmd/gateway/

WORKDIR /app/cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /gateway .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /gateway /gateway

EXPOSE 80

ENTRYPOINT ["/gateway"]

FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/wallet/go.mod cmd/wallet/go.sum cmd/wallet/

WORKDIR /app/cmd/wallet
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/wallet/ cmd/wallet/

WORKDIR /app/cmd/wallet
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /wallet .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /wallet /wallet

ENTRYPOINT ["/wallet"]

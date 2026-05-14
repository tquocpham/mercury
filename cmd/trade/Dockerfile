FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/trade/go.mod cmd/trade/go.sum cmd/trade/

WORKDIR /app/cmd/trade
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/trade/ cmd/trade/

WORKDIR /app/cmd/trade
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /trade .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /trade /trade

ENTRYPOINT ["/trade"]

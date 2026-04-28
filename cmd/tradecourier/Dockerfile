FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/tradecourier/go.mod cmd/tradecourier/go.sum cmd/tradecourier/

WORKDIR /app/cmd/tradecourier
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/tradecourier/ cmd/tradecourier/

WORKDIR /app/cmd/tradecourier
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /tradecourier .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /tradecourier /tradecourier

ENTRYPOINT ["/tradecourier"]

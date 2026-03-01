FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/notifier/go.mod cmd/notifier/go.sum cmd/notifier/

WORKDIR /app/cmd/notifier
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/notifier/ cmd/notifier/

WORKDIR /app/cmd/notifier
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /notifier .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /notifier /notifier

EXPOSE 80

ENTRYPOINT ["/notifier"]

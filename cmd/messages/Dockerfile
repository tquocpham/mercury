FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/messages/go.mod cmd/messages/go.sum cmd/messages/

WORKDIR /app/cmd/messages
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/messages/ cmd/messages/

WORKDIR /app/cmd/messages
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /messages .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /messages /messages

EXPOSE 80

ENTRYPOINT ["/messages"]

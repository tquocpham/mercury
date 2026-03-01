FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/publisher/go.mod cmd/publisher/go.sum cmd/publisher/

WORKDIR /app/cmd/publisher
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/publisher/ cmd/publisher/

WORKDIR /app/cmd/publisher
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /publisher .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /publisher /publisher

EXPOSE 80

ENTRYPOINT ["/publisher"]

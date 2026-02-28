FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/query/go.mod cmd/query/go.sum cmd/query/

WORKDIR /app/cmd/query
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/query/ cmd/query/

WORKDIR /app/cmd/query
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /query .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /query /query

EXPOSE 80

ENTRYPOINT ["/query"]

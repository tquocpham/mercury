FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/subscriber/go.mod cmd/subscriber/go.sum cmd/subscriber/

WORKDIR /app/cmd/subscriber
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/subscriber/ cmd/subscriber/

WORKDIR /app/cmd/subscriber
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /subscriber .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /subscriber /subscriber

EXPOSE 80

ENTRYPOINT ["/subscriber"]

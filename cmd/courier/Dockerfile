FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/courier/go.mod cmd/courier/go.sum cmd/courier/

WORKDIR /app/cmd/courier
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/courier/ cmd/courier/

WORKDIR /app/cmd/courier
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /courier .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /courier /courier

ENTRYPOINT ["/courier"]

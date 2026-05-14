FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/portal/go.mod cmd/portal/go.sum cmd/portal/

WORKDIR /app/cmd/portal
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/portal/ cmd/portal/

WORKDIR /app/cmd/portal
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /portal .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /portal /portal

ENTRYPOINT ["/portal"]

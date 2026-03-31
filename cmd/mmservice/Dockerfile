FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/mmservice/go.mod cmd/mmservice/go.sum cmd/mmservice/

WORKDIR /app/cmd/mmservice
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/mmservice/ cmd/mmservice/

WORKDIR /app/cmd/mmservice
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /mmservice .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /mmservice /mmservice

ENTRYPOINT ["/mmservice"]

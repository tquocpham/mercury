FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/gatewaypriv/go.mod cmd/gatewaypriv/go.sum cmd/gatewaypriv/

WORKDIR /app/cmd/gatewaypriv
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/gatewaypriv/ cmd/gatewaypriv/

WORKDIR /app/cmd/gatewaypriv
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /gatewaypriv .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /gatewaypriv /gatewaypriv

EXPOSE 80

ENTRYPOINT ["/gatewaypriv"]

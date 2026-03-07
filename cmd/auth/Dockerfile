FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/auth/go.mod cmd/auth/go.sum cmd/auth/

WORKDIR /app/cmd/auth
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/auth/ cmd/auth/

WORKDIR /app/cmd/auth
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /auth .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /auth /auth

EXPOSE 80

ENTRYPOINT ["/auth"]

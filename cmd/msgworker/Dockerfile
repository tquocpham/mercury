FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/msgworker/go.mod cmd/msgworker/go.sum cmd/msgworker/

WORKDIR /app/cmd/msgworker
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/msgworker/ cmd/msgworker/

WORKDIR /app/cmd/msgworker
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /msgworker .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /msgworker /msgworker

ENTRYPOINT ["/msgworker"]

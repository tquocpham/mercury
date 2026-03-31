FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY cmd/mmsolver/go.mod cmd/mmsolver/go.sum cmd/mmsolver/

WORKDIR /app/cmd/mmsolver
RUN GOWORK=off go mod download

WORKDIR /app
COPY pkg/ pkg/
COPY cmd/mmsolver/ cmd/mmsolver/

WORKDIR /app/cmd/mmsolver
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build -o /mmsolver .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /mmsolver /mmsolver

ENTRYPOINT ["/mmsolver"]

# syntax=docker/dockerfile:1

FROM golang:1.26.4-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod ./
COPY go.sum* ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /ktrace \
    ./cmd/ktrace

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

ENV HOME=/root

COPY --from=builder /ktrace /usr/local/bin/ktrace

WORKDIR /root

ENTRYPOINT ["ktrace"]

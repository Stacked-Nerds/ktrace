# syntax=docker/dockerfile:1

FROM golang:1.26.4-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod ./
COPY go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /ktrace \
    ./cmd/ktrace

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 65532 ktrace

COPY --from=builder /ktrace /usr/local/bin/ktrace

USER ktrace
WORKDIR /home/ktrace

ENTRYPOINT ["ktrace"]

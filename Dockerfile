# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /bot ./cmd/bot

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && update-ca-certificates \
    && adduser -D -u 1000 -h /app appuser

WORKDIR /app

COPY --from=builder /bot /app/bot

RUN mkdir -p /app/data && chown -R appuser:appuser /app

USER appuser

ENV DB_PATH=/app/data/investment.db \
    TZ=UTC

VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD pidof bot || exit 1

ENTRYPOINT ["/app/bot"]

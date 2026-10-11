# syntax=docker/dockerfile:1

# ── builder ──────────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

# BINARY selects which cmd/* to compile (api | worker | transcoder | dashboard).
ARG BINARY=api

WORKDIR /app

# Cache module downloads separately from source so they survive code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/service ./cmd/${BINARY}

# ── runtime ──────────────────────────────────────────────────────────────────
FROM alpine:3.21

# ffmpeg is required by the worker and transcoder services.
# It is present in all images to keep a single runtime base layer.
RUN apk add --no-cache ffmpeg ca-certificates tzdata

COPY --from=builder /bin/service /bin/service

# Expose the default API port; override with PORT env var if needed.
EXPOSE 3001

CMD ["/bin/service"]

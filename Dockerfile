# syntax=docker/dockerfile:1
#
# Multi-stage build for Our Lady of the Valley Parish.
# Builder → debian-slim runtime, non-root user, uploads dir mounted as a volume.

FROM golang:1.26-trixie AS build

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .

# ---------------- Runtime ----------------
FROM debian:trixie-slim

# Create non-root user up front so the uploads dir can be owned correctly.
RUN groupadd --system --gid 1001 app \
    && useradd  --system --uid 1001 --gid app --shell /bin/false app

WORKDIR /app

# Trust anchors + tz data from the builder (lets https / time-zone loads work).
COPY --from=build /etc/ssl/certs /etc/ssl/certs
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo

# Binary + templates / static assets.
COPY --from=build /app/main /app/main
COPY --from=build /app/web  /app/web

# Pre-create the uploads tree and hand it to the app user so writes work even
# when the host volume mount is empty.
RUN mkdir -p /app/web/static/uploads/legacy \
    && chown -R app:app /app

USER app

ENV GIN_MODE=release
EXPOSE 8080

# Writable upload tree should be mounted as a volume so files survive
# container restarts. Easypanel: mount a named volume at this path.
VOLUME ["/app/web/static/uploads"]

CMD ["./main"]

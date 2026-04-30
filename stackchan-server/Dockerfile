ARG BUILD_FROM
FROM golang:1.23-alpine AS builder

WORKDIR /build
# Copy the Go server source
COPY server/ .
RUN go build -o stackchan-server .

# ── Runtime image (HA base image) ──────────────────────────────────────────
FROM $BUILD_FROM

WORKDIR /app
COPY --from=builder /build/stackchan-server .
COPY addon/run.sh /run.sh
RUN chmod +x /run.sh

CMD ["/run.sh"]

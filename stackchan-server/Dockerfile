ARG BUILD_FROM
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev opus-dev pkg-config

WORKDIR /build
# Copy the Go server source
COPY server/ .
RUN CGO_ENABLED=1 go build -o stackchan-server .

# ── Runtime image (HA base image) ──────────────────────────────────────────
FROM $BUILD_FROM

RUN apk add --no-cache opus

WORKDIR /app
COPY --from=builder /build/stackchan-server .
COPY run.sh /run.sh
RUN chmod +x /run.sh

CMD ["/run.sh"]

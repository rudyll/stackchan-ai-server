ARG BUILD_FROM=alpine:3.23
FROM golang:1.26-alpine3.23 AS builder

RUN apk add --no-cache gcc musl-dev opus-dev opusfile-dev pkgconf

WORKDIR /build
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
RUN CGO_ENABLED=1 go build -mod=readonly -trimpath -o stackchan-server .

# ── Runtime: plain Alpine from Docker Hub (avoids ghcr.io) ─────────────────
FROM ${BUILD_FROM}

RUN apk add --no-cache opus opusfile jq bash ca-certificates

WORKDIR /app
COPY --from=builder /build/stackchan-server .
COPY run.sh /run.sh
COPY standalone.sh /standalone.sh
RUN chmod +x /run.sh /standalone.sh

CMD ["/run.sh"]

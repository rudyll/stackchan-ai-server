FROM golang:1.26-alpine AS builder

RUN apk add --no-cache gcc musl-dev opus-dev opusfile-dev pkgconf

WORKDIR /build
COPY server/ .
RUN go mod tidy && CGO_ENABLED=1 go build -o stackchan-server .

# ── Runtime: plain Alpine from Docker Hub (avoids ghcr.io) ─────────────────
FROM alpine:3.20

RUN apk add --no-cache opus opusfile jq bash ca-certificates

WORKDIR /app
COPY --from=builder /build/stackchan-server .
COPY run.sh /run.sh
RUN chmod +x /run.sh

CMD ["/run.sh"]

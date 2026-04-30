ARG BUILD_FROM
FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY server/ .
RUN go build -o stackchan-server .

# Runtime: HA base image (includes bashio for run.sh)
FROM $BUILD_FROM

WORKDIR /app
COPY --from=builder /build/stackchan-server .
COPY run.sh /run.sh
RUN chmod +x /run.sh

CMD ["/run.sh"]

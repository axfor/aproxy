FROM golang:1.21-alpine AS builder

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /app

RUN apk add --no-cache git make

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN GOEXPERIMENT=greenteagc go build \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o aproxy \
    ./cmd/aproxy

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/aproxy /usr/local/bin/aproxy
COPY configs/config.yaml /app/config.yaml

RUN addgroup -g 1000 proxy && \
    adduser -D -u 1000 -G proxy proxy && \
    chown -R proxy:proxy /app

USER proxy

EXPOSE 3306 9090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:9090/health || exit 1

ENTRYPOINT ["/usr/local/bin/aproxy"]
CMD ["-config", "/app/config.yaml"]

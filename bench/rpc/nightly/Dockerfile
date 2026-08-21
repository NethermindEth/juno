# syntax=docker/dockerfile:1.7.0

FROM golang:1.26-bookworm AS builder

WORKDIR /src
COPY go.mod go.sum ./
COPY starknet-p2p-specs/go.mod starknet-p2p-specs/go.mod
RUN go mod download

COPY Makefile ./
COPY bench/rpc/cmd/corpus-gen/ bench/rpc/cmd/corpus-gen/
RUN CGO_ENABLED=0 make corpus-gen

FROM docker.io/grafana/k6:1.7.1@sha256:4fd3a694926b064d3491d9b02b01cde886583c4931f1223816e3d9a7bdfa7e0f

USER root
RUN apk add --no-cache bash curl jq && \
    mkdir -p /corpus /results && \
    chown k6:k6 /corpus /results

WORKDIR /app
COPY --from=builder /src/build/corpus-gen build/corpus-gen
COPY bench/rpc/nightly/nightly.sh bench/rpc/nightly/ci.json bench/rpc/nightly/
COPY bench/rpc/nightly/internal/ bench/rpc/nightly/internal/
COPY bench/rpc/run.js bench/rpc/run.js
COPY bench/rpc/corpus/nightly.json bench/rpc/corpus/nightly.json

USER k6
ENTRYPOINT ["/app/bench/rpc/nightly/nightly.sh"]

FROM golang:1.26-alpine AS runner-builder

WORKDIR /src
COPY go.mod go.sum ./
COPY bench/rpc/cmd/runner ./bench/rpc/cmd/runner
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/runner \
    ./bench/rpc/cmd/runner

FROM docker.io/grafana/k6:1.7.1@sha256:4fd3a694926b064d3491d9b02b01cde886583c4931f1223816e3d9a7bdfa7e0f

ARG JUNO_COMMIT
LABEL org.opencontainers.image.revision=${JUNO_COMMIT}

USER root
RUN apk add --no-cache ca-certificates curl jq

WORKDIR /bench/rpc
COPY --chown=12345:12345 \
    bench/rpc/run.js \
    bench/rpc/summary-metrics.jq \
    bench/rpc/throughput.js \
    ./
COPY --chown=12345:12345 bench/rpc/runner.sh ./runner.sh
COPY --from=runner-builder --chown=12345:12345 /out/runner ./runner
COPY --chown=12345:12345 \
    bench/rpc/corpus/v0_10/getTransactionByHash.json \
    bench/rpc/corpus/v0_10/getTransactionByHash.json.sha256 \
    ./corpus/v0_10/
RUN printf '%s\n' "${JUNO_COMMIT}" | grep -Eq '^[0-9a-f]{40}$' && \
    printf '%s\n' "${JUNO_COMMIT}" > /bench/rpc/juno-commit && \
    chmod 0444 /bench/rpc/juno-commit && \
    cd corpus/v0_10 && sha256sum -c getTransactionByHash.json.sha256 && \
    chmod 0555 /bench/rpc/runner /bench/rpc/runner.sh && \
    mkdir -p /results && chown 12345:12345 /results

USER 12345:12345
ENTRYPOINT ["/bench/rpc/runner"]

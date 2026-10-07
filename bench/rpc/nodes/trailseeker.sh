#!/usr/bin/env bash
set -euo pipefail

: "${NODE_BIN:?NODE_BIN is required}"
: "${NODE_DB:?NODE_DB is required}"
: "${NODE_NETWORK:=mainnet}"

exec "$NODE_BIN" \
  --data-directory "$NODE_DB" \
  --network "$NODE_NETWORK" \
  --sync.enable false \
  --rpc.batch-concurrency-limit 100 \
  --max-rpc-connections 4096 \
  --http-rpc 127.0.0.1:9545

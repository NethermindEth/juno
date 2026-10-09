#!/usr/bin/env bash
set -euo pipefail

: "${NODE_BIN:?NODE_BIN is required}"
: "${NODE_DB:?NODE_DB is required}"
: "${NODE_NETWORK:=mainnet}"

exec "$NODE_BIN" \
  --db-path "$NODE_DB" \
  --network "$NODE_NETWORK" \
  --disable-sync \
  --disable-l1-verification \
  --max-vm-queue 16384 \
  --http \
  --http-port 6060

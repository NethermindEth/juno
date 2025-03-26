#!/bin/bash
set -euo pipefail

JUNO_LOG="$HOME/juno-benchmark/juno.log"
BLOCK_TARGET=10000
REPORT_FILE="$HOME/juno-benchmark/sync_report.txt"
HTTP_ENDPOINT="http://localhost:6060/v0_8"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$REPORT_FILE"
}

get_current_block_from_log() {
    grep 'Imported block' "$JUNO_LOG" | tail -1 | grep -oP 'number=\K[0-9]+' || echo "0"
}

get_current_block_from_api() {
    local result
    result=$(curl -s "$HTTP_ENDPOINT" \
        --header 'Content-Type: application/json' \
        --data '{
            "jsonrpc": "2.0",
            "method": "starknet_getBlockWithTx",
            "params": {
                "block_id": "latest"
            },
            "id": 1
        }' | grep -o '"block_number":[^,]*' | cut -d':' -f2)
    
    echo "${result:-0}"
}

echo "Juno Sync Report - $(date)" > "$REPORT_FILE"
echo "--------------------------------" >> "$REPORT_FILE"

START_TIME=$(date +%s)
log "Sync started at $(date -u)"
log "Target block: $BLOCK_TARGET"

INITIAL_BLOCK=$(get_current_block_from_log)
log "Starting from block: $INITIAL_BLOCK"

while true; do
    CURRENT_BLOCK=$(get_current_block_from_api)
    
    if [[ "$CURRENT_BLOCK" == "0" ]]; then
        CURRENT_BLOCK=$(get_current_block_from_log)
    fi
    
    echo "Current block: $CURRENT_BLOCK"
    
    if [[ "$CURRENT_BLOCK" -ge "$BLOCK_TARGET" ]]; then
        END_TIME=$(date +%s)
        SYNC_DURATION=$((END_TIME - START_TIME))
        SYNC_MINUTES=$(echo "scale=2; $SYNC_DURATION / 60" | bc)
        
        log "Sync completed in: $SYNC_MINUTES minutes"
        log "Blocks synced: $((CURRENT_BLOCK - INITIAL_BLOCK))"
        log "Average speed: $(echo "scale=2; ($CURRENT_BLOCK - $INITIAL_BLOCK) / $SYNC_DURATION" | bc) blocks/second"
        
        log "Sync report saved to: $REPORT_FILE"
        break
    fi
done
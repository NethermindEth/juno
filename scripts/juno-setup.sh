#!/bin/bash
set -euo pipefail

JUNO_VERSION="${1:-latest}"
SNAPSHOT_URL="${SNAPSHOT_URL:-https://juno-snapshots.nethermind.io/files/sepolia/latest}"
WORK_DIR="$HOME/juno-benchmark"
DB_DIR="$WORK_DIR/db"
JUNO_LOG="$WORK_DIR/juno.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

cd "$WORK_DIR"

log "Downloading snapshot from $SNAPSHOT_URL..."
wget -q -c --tries=3 --retry-connrefused --waitretry=5 --timeout=10 "$SNAPSHOT_URL" -O snapshot.tar
if [ ! -s snapshot.tar ]; then
    log "Snapshot download failed or file is empty"
    exit 1
fi
tar -xf snapshot.tar -C $DB_DIR && rm snapshot.tar

#log "Downloading Juno version $JUNO_VERSION..."
#if [ "$JUNO_VERSION" = "latest" ]; then
#    JUNO_VERSION=$(curl -s https://api.github.com/repos/NethermindEth/juno/releases/latest | grep -o '"tag_name": ".*"' | cut -d'"' -f4)
#    log "Latest version resolved to $JUNO_VERSION."
#fi

#JUNO_BINARY_URL="https://github.com/NethermindEth/juno/releases/download/$JUNO_VERSION/juno-$JUNO_VERSION-linux-amd64.zip"

#wget -q "$JUNO_BINARY_URL" -O juno.zip
#unzip -q juno.zip && rm juno.zip
#mv juno-*-linux-amd64 juno
#chmod +x juno
#./juno --version || (log "Juno binary verification failed" && exit 1)

log "Starting Juno..."
nohup "$WORK_DIR/juno/build/juno" \
    --db-path="$DB_DIR" \
    --disable-l1-verification \
    --network=sepolia \
    --http \
    --http-host=127.0.0.1 \
    --http-port=6060 \
    --metrics \
    --metrics-host=127.0.0.1 \
    --metrics-port=9090 \
    > "$JUNO_LOG" 2>&1 &

sleep 5
if pgrep -x "juno" > /dev/null; then
    log "Juno started successfully."
else
    log "Juno failed to start. Check log:"
    tail -n 20 "$JUNO_LOG"
    exit 1
fi

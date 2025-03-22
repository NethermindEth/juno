#!/bin/bash
set -euo pipefail

JUNO_VERSION="${1:-latest}"
SNAPSHOT_URL="${SNAPSHOT_URL:-https://juno-snapshots.nethermind.io/files/sepolia/latest}"
WORK_DIR="$HOME/juno-benchmark"
JUNO_LOG="$WORK_DIR/juno.log"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

cd "$WORK_DIR"

log "Downloading snapshot from $SNAPSHOT_URL..."
wget -q -c --tries=5 --retry-connrefused --waitretry=10 --timeout=60 "$SNAPSHOT_URL" -O snapshot.tar
if [ ! -s snapshot.tar ]; then
    log "Snapshot download failed or file is empty"
    exit 1
fi
tar -xzf snapshot.tar && rm snapshot.tar
SNAPSHOT_DIR=$(find "$WORK_DIR" -type d -mindepth 1 -maxdepth 1 | head -1)

log "Downloading Juno version $JUNO_VERSION..."
if [ "$JUNO_VERSION" = "latest" ]; then
    JUNO_VERSION=$(curl -s https://api.github.com/repos/NethermindEth/juno/releases/latest | grep -o '"tag_name": ".*"' | cut -d'"' -f4)
    log "Latest version resolved to $JUNO_VERSION."
fi

JUNO_BINARY_URL="https://github.com/NethermindEth/juno/releases/download/$JUNO_VERSION/juno-$JUNO_VERSION-linux-amd64.zip"

wget -q "$JUNO_BINARY_URL" -O juno.zip
unzip -q juno.zip && rm juno.zip
mv juno-*-linux-amd64 juno
chmod +x juno
./juno --version || (log "Juno binary verification failed" && exit 1)

log "Starting Juno..."
nohup "$WORK_DIR/juno" \
    --db-path="$WORK_DIR/db" \
    --snapshot-path="$SNAPSHOT_DIR" \
    --sync-mode=full \
    --disable-l1-verification=true \
    --target-block=10000 \
    > "$JUNO_LOG" 2>&1 &

sleep 5
if pgrep -x "juno" > /dev/null; then
    log "Juno started successfully."
else
    log "Juno failed to start. Check log:"
    tail -n 20 "$JUNO_LOG"
    exit 1
fi

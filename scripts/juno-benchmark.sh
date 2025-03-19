#!/bin/bash
# Automate EC2 login, snapshot download, and running Juno

set -e

EC2_HOST="${EC2_HOST:-3.126.138.194}"
EC2_USER="${EC2_USER:-ubuntu}"
EC2_KEY_PATH="${EC2_KEY_PATH:-~/.ssh/ec2_key.pem}"
JUNO_VERSION="${1:-latest}"
SNAPSHOT_URL="${SNAPSHOT_URL:-https://juno-snapshots.nethermind.io/files/mainnet/latest}"

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

validate_config() {
    local missing_vars=0
    
    if [ -z "$EC2_HOST" ]; then
        log "Error: EC2_HOST environment variable is not set"
        missing_vars=1
    fi
    
    if [ -z "$SNAPSHOT_URL" ]; then
        log "Error: SNAPSHOT_URL environment variable is not set"
        missing_vars=1
    fi
    
    if [ ! -f "$EC2_KEY_PATH" ]; then
        log "Error: SSH key not found at $EC2_KEY_PATH"
        missing_vars=1
    fi
    
    if [ $missing_vars -ne 0 ]; then
        log "Please set the required environment variables and try again"
        exit 1
    fi
}

run_remote() {
    log "Running remote command: $1"
    ssh -i "$EC2_KEY_PATH" -o StrictHostKeyChecking=no "$EC2_USER@$EC2_HOST" "$1"
}

copy_to_remote() {
    log "Copying $1 to $EC2_USER@$EC2_HOST:$2"
    scp -i "$EC2_KEY_PATH" -o StrictHostKeyChecking=no "$1" "$EC2_USER@$EC2_HOST:$2"
}

test_ssh_connection() {
    log "Testing SSH connection to $EC2_HOST..."
    if ! ssh -i "$EC2_KEY_PATH" -o StrictHostKeyChecking=no -o ConnectTimeout=10 "$EC2_USER@$EC2_HOST" "echo 'Connection successful'"; then
        log "Error: Failed to connect to EC2 instance"
        exit 1
    fi
    log "SSH connection successful"
}

setup_working_dir() {
    log "Setting up clean working directory on EC2..."
    run_remote "pkill -x juno || true"
    run_remote "rm -rf ~/juno-benchmark && mkdir -p ~/juno-benchmark"
    log "Checking available disk space on EC2:"
    run_remote "df -h | grep -E '/$|/home'"
    log "Working directory created"
}

download_snapshot() {
    log "Downloading Starknet snapshot from $SNAPSHOT_URL..."
    run_remote "cd ~/juno-benchmark && wget --progress=dot:giga $SNAPSHOT_URL -O snapshot.tar.gz"
    if ! run_remote "cd ~/juno-benchmark && [ -s snapshot.tar.gz ]"; then
        log "Error: Snapshot download failed or file is empty"
        exit 1
    fi
    log "Extracting snapshot..."
    run_remote "cd ~/juno-benchmark && tar -xzf snapshot.tar.gz && rm snapshot.tar.gz"
    log "Verifying snapshot extraction..."
    run_remote "cd ~/juno-benchmark && find . -type d -mindepth 1 -maxdepth 1"
    log "Snapshot downloaded and extracted"
}

download_juno() {
    log "Downloading Juno binary version $JUNO_VERSION..."
    
    if [ "$JUNO_VERSION" = "latest" ]; then
        JUNO_VERSION=$(curl -s https://api.github.com/repos/NethermindEth/juno/releases/latest | grep -o '"tag_name": ".*"' | cut -d'"' -f4)
        log "Latest version determined to be: $JUNO_VERSION"
    fi
    
    run_remote "cd ~/juno-benchmark && wget --progress=dot:mega https://github.com/NethermindEth/juno/releases/download/$JUNO_VERSION/juno-$JUNO_VERSION-linux-amd64.zip -O juno.zip"
    run_remote "cd ~/juno-benchmark && unzip -q juno.zip && rm juno.zip"
    run_remote "cd ~/juno-benchmark && mv juno-* juno"
    run_remote "cd ~/juno-benchmark && chmod +x juno"
    log "Verifying Juno binary..."
    run_remote "cd ~/juno-benchmark && ./juno version || ./juno --version"
    log "Juno binary $JUNO_VERSION downloaded and made executable"
}

run_juno() {
    log "Setting up to run Juno..."
    SNAPSHOT_DIRS=$(run_remote "cd ~/juno-benchmark && find . -type d -not -path './.*' | grep -v '^.$'")
    log "Found directories: $SNAPSHOT_DIRS"
    if echo "$SNAPSHOT_DIRS" | grep -q "mainnet"; then
        SNAPSHOT_DIR="./mainnet"
    elif echo "$SNAPSHOT_DIRS" | grep -q "data"; then
        SNAPSHOT_DIR="./data"
    else
        SNAPSHOT_DIR=$(echo "$SNAPSHOT_DIRS" | head -1)
    fi
    log "Using snapshot directory: $SNAPSHOT_DIR"
    cat > juno_config.json <<EOF
{
  "http-port": 6060,
  "db-path": "./data",
  "snapshot-path": "$SNAPSHOT_DIR",
  "log-level": "info",
  "sync-mode": "full",
  "disable-etc-sync": false,
  "target-block": 10000
}
EOF
    log "Created Juno configuration"
    copy_to_remote "juno_config.json" "~/juno-benchmark/config.json"
    log "Starting Juno..."
    run_remote "cd ~/juno-benchmark && nohup ./juno --config=config.json > juno.log 2>&1 &"
    sleep 5
    log "Checking if Juno is running:"
    run_remote "ps aux | grep juno | grep -v grep || echo 'Juno process not found'"
    log "Juno started. Initial log output:"
    run_remote "tail -n 20 ~/juno-benchmark/juno.log"
}

main() {
    log "Starting Juno benchmarking setup - Version $JUNO_VERSION"
    validate_config
    test_ssh_connection
    setup_working_dir
    download_snapshot
    download_juno
    run_juno
    log "Initial setup completed successfully"
    log "Juno is now running on the EC2 instance"
    log "To monitor progress, use: ssh -i $EC2_KEY_PATH $EC2_USER@$EC2_HOST 'tail -f ~/juno-benchmark/juno.log'"
}

main "$1"


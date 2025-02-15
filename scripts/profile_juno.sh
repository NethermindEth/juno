#!/bin/bash

JUNO_PORT=6060      # Juno RPC port
PPROF_PORT=6062     # pprof profiling port

# Default values
DEFAULT_PROFILE="profile"
DEFAULT_PROFILE_DURATION=30
DEFAULT_REQUEST_FILE="scripts/payload/getBlockWithTxs_0.json"
DEFAULT_NETWORK="sepolia"
DEFAULT_OUTPUT_FILE="trace.out"
DEFAULT_DB_PATH="./p2p-dbs/feedernode"

# Read values from command-line arguments or use defaults
PROFILE=${1:-$DEFAULT_PROFILE}
PROFILE_DURATION=${2:-$DEFAULT_PROFILE_DURATION}
REQUEST_FILE=${3:-$DEFAULT_REQUEST_FILE}
NETWORK=${4:-$DEFAULT_NETWORK}
OUTPUT_FILE=${5:-$DEFAULT_OUTPUT_FILE}
DB_PATH=${6:-$DEFAULT_DB_PATH}

# Configurations
JUNO_BINARY="./build/juno"
JUNO_ARGS="--network=$NETWORK \
        --log-level=debug \
        --db-path=$DB_PATH \
        --p2p \
        --p2p-feeder-node \
        --p2p-addr=/ip4/0.0.0.0/tcp/7777 \
        --p2p-private-key="5f6cdc3aebcc74af494df054876100368ef6126e3a33fa65b90c765b381ffc37a0a63bbeeefab0740f24a6a38dabb513b9233254ad0020c721c23e69bc820089" \
        --http \
        --http-host "0.0.0.0" \
        --http-port "$JUNO_PORT" \
        --pprof \
        --pprof-port "$PPROF_PORT" \
        --disable-l1-verification"

# Ensure the request file exists if not set to "none"
if [ "$REQUEST_FILE" != "none" ] && [ ! -f "$REQUEST_FILE" ]; then
    echo "Error: Request file '$REQUEST_FILE' not found."
    exit 1
fi

if [ "$REQUEST_FILE" != "none" ]; then
    REQUEST_PAYLOAD=$(cat "$REQUEST_FILE")
fi

echo "Using profiling duration: $PROFILE_DURATION seconds"
echo "Using request file: $REQUEST_FILE"

# 1. Start Juno in a new terminal session and log output to juno.log
echo "Starting Juno..."
tmux new-session -d -s juno_session "bash -c '$JUNO_BINARY $JUNO_ARGS > juno.log 2>&1'"

# 2. Wait for Juno to be ready
echo "Waiting for Juno to start..."
MAX_WAIT=60
WAIT_TIME=0
while ! curl --silent --fail --location 'http://localhost:6060' \
--header 'Content-Type: application/json' \
--data '{
    "jsonrpc":"2.0",
    "method":"juno_version",
    "id":1
}'; do
    if ! tmux has-session -t juno_session 2>/dev/null; then
        echo "Error: Juno crashed during startup."
        exit 1
    fi
    if [ $WAIT_TIME -ge $MAX_WAIT ]; then
        echo "Juno did not start within $MAX_WAIT seconds. Exiting."
        tmux kill-session -t juno_session
        exit 1
    fi
    echo "Still waiting for Juno... ($WAIT_TIME seconds)"
    sleep 2
    ((WAIT_TIME+=2))
done
echo "Juno is up and running."

# 3. Start CPU profiling in the background
echo "Capturing profile for $PROFILE_DURATION seconds..."
curl -o $OUTPUT_FILE "http://localhost:$PPROF_PORT/debug/pprof/$PROFILE?seconds=$PROFILE_DURATION" &

if [ "$REQUEST_FILE" != "none" ]; then
    # 4. Send requests and count responses
    echo "Sending requests for $PROFILE_DURATION seconds..."
    END_TIME=$((SECONDS + PROFILE_DURATION))
    TOTAL_REQUESTS=0
    SUCCESSFUL_REQUESTS=0

    while [ $SECONDS -lt $END_TIME ]; do
        RESPONSE=$(curl --silent --location "http://localhost:$JUNO_PORT" \
            --header "Content-Type: application/json" \
            --data "$REQUEST_PAYLOAD")

        ((TOTAL_REQUESTS++))

        # Check if response contains a result field
        if echo "$RESPONSE" | jq -e '.result' > /dev/null 2>&1; then
            ((SUCCESSFUL_REQUESTS++))
        else
            echo "Error in response: $RESPONSE"
        fi
    done

    # Print request statistics
    echo "Total requests sent: $TOTAL_REQUESTS"
    echo "Successful requests: $SUCCESSFUL_REQUESTS"
fi

# Wait for profiling and requests to complete
wait
echo "CPU profile saved to trace.out"

# 5. Stop Juno
echo "Stopping Juno..."
tmux kill-session -t juno_session
echo "Juno stopped."

# 6. Analyze the profile (optional)
# echo "Run 'go tool pprof trace.out' to analyze the profile."
echo "Run 'go tool pprof -http=:3000 $OUTPUT_FILE' to analyze the profile."

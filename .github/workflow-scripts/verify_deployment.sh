#!/bin/bash

URL=$1
EXPECTED_VERSION=$2
MAX_ATTEMPTS=30
SLEEP_DURATION=10

for i in $(seq 1 $MAX_ATTEMPTS); do
  DEPLOYED_VERSION=$(curl -s -X POST "$URL" -H "Content-Type: application/json" -d '{"method": "juno_version", "jsonrpc": "2.0", "id": 0}' | jq -r '.result')
  echo "Attempt $i: Deployed version $DEPLOYED_VERSION"
  if [[ "$DEPLOYED_VERSION" == "$EXPECTED_VERSION" ]]; then
    echo "Deployment version matches the image tag."
    exit 0
  fi
  sleep $SLEEP_DURATION
done

echo "Deployment version did not match the image tag within the expected time frame."
exit 1

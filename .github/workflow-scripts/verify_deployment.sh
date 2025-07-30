#!/bin/bash

set -ux

BASE_URL=$1
AUTH_KEY=$2
EXPECTED_VERSION=$3
URL="${BASE_URL}?apikey=${AUTH_KEY}"
MAX_ATTEMPTS=30
SLEEP_DURATION=10

for i in $(seq 1 $MAX_ATTEMPTS); do
  DEPLOYED_VERSION=$(curl -s -X POST "$URL" -H "Content-Type: application/json" -d '{"method": "juno_version", "jsonrpc": "2.0", "id": 0}' | jq -r '.result')
  IS_READY=$(curl -s -o /dev/null -w "%{http_code}" -v "${BASE_URL}/ready?apikey=${AUTH_KEY}")
  echo "Attempt $i: IS_READY response code: $IS_READY"
  echo "Attempt $i: Deployed version $DEPLOYED_VERSION"
  if [[ "$DEPLOYED_VERSION" == "$EXPECTED_VERSION" && "$IS_READY" == "200" ]]; then
    echo "Deployment version matches the image tag and Node is ready"
    exit 0
  fi

  sleep $SLEEP_DURATION
done

echo "Deployment version did not match the image tag within the expected time frame."
exit 1

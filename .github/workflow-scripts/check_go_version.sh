#!/bin/bash
# Checks that the release builds use the latest patch release of the Go minor version in go.mod.

set -euo pipefail

# Matches each Dockerfile stage that builds from a golang image.
GOLANG_FROM='^FROM .*golang:'

TOOLCHAIN=$(sed -nE 's/^toolchain go([0-9]+\.[0-9]+\.[0-9]+)$/\1/p' go.mod)
if [[ -z "$TOOLCHAIN" ]]; then
  echo "::error file=go.mod::go.mod has no 'toolchain goX.Y.Z' line"
  exit 1
fi
echo "go.mod toolchain: go${TOOLCHAIN}"

FAILED=0
while IFS=: read -r FILE FROM; do
  [[ "$FROM" =~ golang:([^-@ ]+) ]]
  TAG="${BASH_REMATCH[1]}"
  if [[ "$TAG" != "$TOOLCHAIN" ]]; then
    echo "::error file=${FILE}::${FILE} uses golang:${TAG}, go.mod toolchain is go${TOOLCHAIN}"
    FAILED=1
  fi
done < <(git grep "$GOLANG_FROM" -- '*Dockerfile*')

MINOR="${TOOLCHAIN%.*}"
if ! RELEASES=$(curl -fsS --retry 3 --max-time 30 'https://go.dev/dl/?mode=json'); then
  echo "::error::Cannot read the Go releases from https://go.dev/dl/"
  exit 1
fi
LATEST=$(jq -r --arg minor "go${MINOR}." '[.[].version | select(startswith($minor))] | first // empty' <<<"$RELEASES")
if [[ -z "$LATEST" ]]; then
  echo "::error::Cannot find the latest go${MINOR}.x release on https://go.dev/dl/"
  exit 1
fi
echo "Latest go${MINOR}.x release: ${LATEST}"

if [[ "$LATEST" != "go${TOOLCHAIN}" ]]; then
  # Docker Hub publishes the golang image some hours after the Go release. Until then the
  # Dockerfiles cannot move to the new patch.
  IMAGE_URL="https://hub.docker.com/v2/repositories/library/golang/tags/${LATEST#go}"
  IMAGE_STATUS=$(curl -sS --retry 3 --max-time 30 -o /dev/null -w '%{http_code}' "$IMAGE_URL") || true
  case "$IMAGE_STATUS" in
    200)
      echo "::error file=go.mod::go${TOOLCHAIN} is not the latest patch release, update to ${LATEST}:"
      echo "  go mod edit -toolchain=${LATEST}"
      echo "  git grep -l '${GOLANG_FROM}' -- '*Dockerfile*' | xargs sed -i 's/golang:${TOOLCHAIN}-/golang:${LATEST#go}-/'"
      FAILED=1
      ;;
    404)
      echo "::warning file=go.mod::${LATEST} is released, but the golang:${LATEST#go} image is not on Docker Hub yet"
      ;;
    *)
      echo "::error::Cannot read ${IMAGE_URL} (HTTP status ${IMAGE_STATUS})"
      FAILED=1
      ;;
  esac
fi

exit "$FAILED"

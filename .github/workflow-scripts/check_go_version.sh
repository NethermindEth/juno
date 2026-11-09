#!/bin/bash
# Checks that the release builds use the latest patch release of the Go minor version in go.mod.

set -euo pipefail

# Matches each Dockerfile stage that builds from a golang image.
GOLANG_FROM='^FROM .*golang:'
# Captures the version of the golang image tag, e.g. 1.27.4 in golang:1.27.4-bookworm.
TAG_RE='golang:([0-9.]+)([-@ ]|$)'

TOOLCHAIN=$(sed -nE 's/^toolchain go([0-9]+\.[0-9]+\.[0-9]+)$/\1/p' go.mod)
if [[ -z "$TOOLCHAIN" ]]; then
  echo "::error file=go.mod::go.mod has no 'toolchain goX.Y.Z' line"
  exit 1
fi
echo "go.mod toolchain: go${TOOLCHAIN}"

# Prints the edits that move go.mod and each Dockerfile to Go version $1.
print_fix() {
  echo "Make these edits:"
  echo "  go mod edit -toolchain=go$1"
  while IFS=: read -r FILE LINE FROM; do
    echo "  ${FILE}:${LINE}  $(sed -E "s/golang:[0-9.]+/golang:$1/" <<<"$FROM")"
  done < <(git grep -n "$GOLANG_FROM" -- '*Dockerfile*')
}

FAILED=0
while IFS=: read -r FILE LINE FROM; do
  if ! [[ "$FROM" =~ $TAG_RE ]]; then
    echo "::error file=${FILE},line=${LINE}::${FILE} has no golang:X.Y.Z tag in '${FROM}'"
    FAILED=1
    continue
  fi
  TAG="${BASH_REMATCH[1]}"
  if [[ "$TAG" != "$TOOLCHAIN" ]]; then
    echo "::error file=${FILE},line=${LINE}::${FILE} uses golang:${TAG}, go.mod toolchain is go${TOOLCHAIN}"
    FAILED=1
  fi
done < <(git grep -n "$GOLANG_FROM" -- '*Dockerfile*')

if [[ "$FAILED" == 1 ]]; then
  print_fix "$TOOLCHAIN"
fi

MINOR="${TOOLCHAIN%.*}"
# include=all also lists the Go minors that are no longer supported, so that a release branch
# on an older minor can still find its latest patch. go.dev lists the newest release first.
if ! RELEASES=$(curl -fsS --retry 3 --max-time 30 'https://go.dev/dl/?mode=json&include=all'); then
  echo "::error::Cannot read the Go releases from https://go.dev/dl/"
  exit 1
fi
LATEST=$(jq -r --arg minor "go${MINOR}." '[.[] | select(.stable) | .version | select(startswith($minor))] | first // empty' <<<"$RELEASES")
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
      echo "::error file=go.mod::go${TOOLCHAIN} is not the latest patch release, update to ${LATEST}"
      print_fix "${LATEST#go}"
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

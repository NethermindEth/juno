#!/bin/sh

set -eu
if (set -o pipefail) 2>/dev/null; then
  set -o pipefail
fi

IMAGE=${1:-juno-rpc-benchmark-runner:test}
readonly RUNNER=/bench/rpc/runner
REPO_ROOT=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
FIXTURE_DIR="$REPO_ROOT/bench/rpc/testdata"
TEST_DIR=$(mktemp -d)
stub_pids=
container_names=

cleanup() {
  for pid in $stub_pids; do
    kill "$pid" 2>/dev/null || true
  done
  for container_name in $container_names; do
    docker rm -f "$container_name" >/dev/null 2>&1 || true
  done
  rm -rf "$TEST_DIR"
}
trap cleanup EXIT INT TERM

docker run --rm --entrypoint sh "$IMAGE" -c 'test -w /results'
IMAGE_COMMIT=$(docker run --rm --entrypoint cat "$IMAGE" /bench/rpc/juno-commit)
if ! printf '%s\n' "$IMAGE_COMMIT" | grep -Eq '^[0-9a-f]{40}$'; then
  echo "runner image contains an invalid source commit" >&2
  exit 1
fi
IMAGE_JUNO_VERSION="sha-$IMAGE_COMMIT"

start_stub() {
  mode=$1
  ready_failures=$2
  juno_version=${3:-$IMAGE_JUNO_VERSION}
  port_file="$TEST_DIR/port-$mode-$ready_failures"
  python3 "$FIXTURE_DIR/stub_server.py" \
    --mode "$mode" \
    --ready-failures "$ready_failures" \
    --juno-version "$juno_version" \
    --port-file "$port_file" \
    > "$TEST_DIR/stub-$mode-$ready_failures.log" 2>&1 &
  pid=$!
  stub_pids="$stub_pids $pid"

  attempts=0
  while [ ! -s "$port_file" ]; do
    attempts=$((attempts + 1))
    if [ "$attempts" -ge 50 ]; then
      echo "stub server did not start" >&2
      exit 1
    fi
    sleep 0.1
  done
  stub_port=$(cat "$port_file")
}

new_results_dir() {
  directory="$TEST_DIR/results-$1"
  mkdir "$directory"
  chmod 0777 "$directory"
  echo "$directory"
}

# run_runner <port> <results-dir> <corpus-dir> [docker arguments...]
run_runner() {
  runner_port=$1
  runner_results=$2
  runner_corpus_dir=$3
  shift 3

  if [ -n "${RUNNER_CONTAINER_NAME:-}" ]; then
    set -- -d --name "$RUNNER_CONTAINER_NAME" "$@"
  else
    set -- --rm "$@"
  fi

  docker run --network host --user 12345:12345 \
    --entrypoint "$RUNNER" \
    -v "$runner_corpus_dir:/corpus:ro" \
    -v "$runner_results:/results" \
    -e NODE_URL="http://127.0.0.1:$runner_port/v0_10" \
    -e READY_URL="http://127.0.0.1:$runner_port/ready/rpc" \
    -e CORPUS_PATH=/corpus/corpus.json \
    -e EXPECTED_CHAIN_ID=0x534e5f4d41494e \
    -e EXPECTED_BLOCK_NUMBER=800000 \
    -e SNAPSHOT_ID=test-snapshot \
    -e SNAPSHOT_SHA256=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
    -e JUNO_IMAGE_DIGEST=sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb \
    -e RUNNER_IMAGE_DIGEST=sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc \
    -e READY_TIMEOUT=10s \
    -e READY_POLL_INTERVAL=1s \
    -e ITERATIONS=2 \
    -e VUS=1 \
    -e CONCURRENCY_DURATION=1s \
    -e THROUGHPUT_DURATION=1s \
    -e RATES=10 \
    -e THROUGHPUT_VUS=3 \
    "$@" \
    "$IMAGE"
}

start_stub ok 2
success_port=$stub_port
success_results=$(new_results_dir success)
run_runner "$success_port" "$success_results" "$FIXTURE_DIR" -e RATES=" 10 "

jq -e --arg juno_version "$IMAGE_JUNO_VERSION" --arg juno_commit "$IMAGE_COMMIT" '
  .status == "passed" and
  (.runId | test("^[0-9]{8}T[0-9]{6}Z-" + $juno_commit[0:12] + "$")) and
  .node.readiness == "passed" and
  .node.blockNumber == 800000 and
  .juno.version == $juno_version and
  .scenarios.single.metrics.failedChecks == 0 and
  .scenarios.single.metrics.requestFailures == 0 and
  .scenarios.single.metrics.vuFailures == 0 and
  .scenarios.single.metrics.droppedIterations == 0 and
  .scenarios.single.metrics.completedIterations == 2 and
  .scenarios.warmup.measured == false and
  .scenarios.warmup.iterations == 200 and
  .scenarios.single.status == "passed" and
  .scenarios.concurrency.status == "passed" and
  .scenarios.throughput.status == "passed" and
  .scenarios.throughput.preAllocatedVUs == 3 and
  .scenarios.throughput.maxVUs == 3 and
  .scenarios.throughput.rates == [10]
' "$success_results/manifest.json" >/dev/null
for scenario in single concurrency throughput; do
  jq -e '.metrics.checks.value == 1 and .metrics.checks.fails == 0' \
    "$success_results/$scenario.json" >/dev/null
done

signal_results=$(new_results_dir signal)
signal_container="juno-rpc-benchmark-signal-$$"
container_names="$container_names $signal_container"
RUNNER_CONTAINER_NAME=$signal_container run_runner \
  "$success_port" "$signal_results" "$FIXTURE_DIR" \
  -e RUN_ID=signal-test -e CONCURRENCY_DURATION=30s >/dev/null

attempts=0
until docker top "$signal_container" 2>/dev/null | grep -F -- '--duration 30s' >/dev/null; do
  attempts=$((attempts + 1))
  if [ "$attempts" -ge 300 ] || [ "$(docker inspect -f '{{.State.Running}}' "$signal_container")" != true ]; then
    docker logs "$signal_container" >&2 || true
    echo "runner did not reach the concurrency stage" >&2
    exit 1
  fi
  sleep 0.1
done
docker stop --time 45 "$signal_container" >/dev/null
test "$(docker inspect -f '{{.State.ExitCode}}' "$signal_container")" = 143
jq -e '
  .status == "failed" and
  .finishedAt != null and
  .failure.stage == "concurrency" and
  .failure.reason == "runner terminated by TERM" and
  .scenarios.concurrency.status == "failed"
' "$signal_results/manifest.json" >/dev/null

start_stub rpc-error 0
rpc_error_port=$stub_port
rpc_error_results=$(new_results_dir rpc-error)
if run_runner "$rpc_error_port" "$rpc_error_results" "$FIXTURE_DIR"; then
  echo "runner unexpectedly accepted failed RPC checks" >&2
  exit 1
fi
jq -e '
  .status == "failed" and
  .failure.stage == "warmup" and
  .scenarios.warmup.metrics.failedChecks > 0 and
  .scenarios.warmup.metrics.requestFailures > 0
' \
  "$rpc_error_results/manifest.json" >/dev/null

echo "benchmark runner integration tests passed"

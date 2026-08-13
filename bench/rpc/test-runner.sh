#!/bin/sh

set -eu
if (set -o pipefail) 2>/dev/null; then
  set -o pipefail
fi

IMAGE=${1:-juno-rpc-benchmark-runner:test}
RUNNER=${2:-/bench/rpc/runner}
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

missing_checks_metrics=$(printf '%s\n' \
  '{"metrics":{"vu_failures":{"values":{"count":3}}}}' \
  | jq -ce -f "$REPO_ROOT/bench/rpc/summary-metrics.jq")
printf '%s\n' "$missing_checks_metrics" | jq -e '
  .failedChecks == 0 and
  .requestFailures == 0 and
  .vuFailures == 3 and
  .droppedIterations == 0 and
  .completedIterations == 0
' >/dev/null

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

# run_runner <port> <results-dir> <corpus-dir> [KEY=VALUE ...]; trailing -e wins.
run_runner() {
  runner_port=$1
  runner_results=$2
  runner_corpus_dir=$3
  shift 3

  override_count=$#
  appended=0
  while [ "$appended" -lt "$override_count" ]; do
    override=$1
    shift
    set -- "$@" -e "$override"
    appended=$((appended + 1))
  done

  docker run --rm --network host --user 12345:12345 \
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
    -e JUNO_IMAGE_DIGEST=sha256:juno \
    -e RUNNER_IMAGE_DIGEST=sha256:runner \
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
run_runner "$success_port" "$success_results" "$FIXTURE_DIR" RATES=" 10 "

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

embedded_corpus_results=$(new_results_dir embedded-corpus)
run_runner "$success_port" "$embedded_corpus_results" "$FIXTURE_DIR" CORPUS_PATH=
jq -e '
  .status == "passed" and
  .corpus.sha256 != null and
  .scenarios.single.status == "passed" and
  .scenarios.concurrency.status == "passed" and
  .scenarios.throughput.status == "passed"
' "$embedded_corpus_results/manifest.json" >/dev/null

# Saturation must pass and report the drops, not fail the run.
saturation_results=$(new_results_dir saturation)
run_runner "$success_port" "$saturation_results" "$FIXTURE_DIR" \
  RATES=100000 THROUGHPUT_VUS=1
jq -e '
  .status == "passed" and
  .scenarios.throughput.status == "passed" and
  .scenarios.throughput.metrics.droppedIterations > 0 and
  .scenarios.throughput.metrics.failedChecks == 0 and
  .scenarios.throughput.metrics.requestFailures == 0
' "$saturation_results/manifest.json" >/dev/null

chain_results=$(new_results_dir chain-mismatch)
if run_runner "$success_port" "$chain_results" "$FIXTURE_DIR" EXPECTED_CHAIN_ID=0xBAD; then
  echo "runner unexpectedly accepted a chain ID mismatch" >&2
  exit 1
fi
jq -e '.status == "failed" and .failure.stage == "target-validation"' \
  "$chain_results/manifest.json" >/dev/null

block_results=$(new_results_dir block-mismatch)
if run_runner "$success_port" "$block_results" "$FIXTURE_DIR" \
  EXPECTED_BLOCK_NUMBER=799999; then
  echo "runner unexpectedly accepted a snapshot head mismatch" >&2
  exit 1
fi
jq -e '
  .status == "failed" and
  .failure.stage == "target-validation" and
  .node.expectedBlockNumber == 799999 and
  .node.blockNumber == 800000
' "$block_results/manifest.json" >/dev/null

invalid_block_results=$(new_results_dir invalid-block)
if run_runner "$success_port" "$invalid_block_results" "$FIXTURE_DIR" \
  EXPECTED_BLOCK_NUMBER=not-a-number; then
  echo "runner unexpectedly accepted a non-numeric expected block number" >&2
  exit 1
fi
jq -e '
  .status == "failed" and
  .failure.stage == "configuration" and
  .node.expectedBlockNumber == null
' "$invalid_block_results/manifest.json" >/dev/null

start_stub ok 0 sha-deadbeef
version_mismatch_port=$stub_port
version_results=$(new_results_dir version-mismatch)
if run_runner "$version_mismatch_port" "$version_results" "$FIXTURE_DIR"; then
  echo "runner unexpectedly accepted a Juno image mismatch" >&2
  exit 1
fi
jq -e '.status == "failed" and .failure.stage == "target-validation"' \
  "$version_results/manifest.json" >/dev/null

bad_corpus_dir="$TEST_DIR/bad-corpus"
mkdir "$bad_corpus_dir"
cp "$FIXTURE_DIR/corpus.json" "$bad_corpus_dir/corpus.json"
printf '%064d  corpus.json\n' 0 > "$bad_corpus_dir/corpus.json.sha256"
checksum_results=$(new_results_dir checksum-mismatch)
if run_runner "$success_port" "$checksum_results" "$bad_corpus_dir"; then
  echo "runner unexpectedly accepted a corrupt corpus" >&2
  exit 1
fi
jq -e '.status == "failed" and .failure.stage == "corpus-validation"' \
  "$checksum_results/manifest.json" >/dev/null

no_meta_corpus_dir="$TEST_DIR/no-meta-corpus"
mkdir "$no_meta_corpus_dir"
jq -c 'del(.meta)' "$FIXTURE_DIR/corpus.json" > "$no_meta_corpus_dir/corpus.json"
(cd "$no_meta_corpus_dir" && sha256sum corpus.json > corpus.json.sha256)
no_meta_results=$(new_results_dir no-meta-corpus)
if run_runner "$success_port" "$no_meta_results" "$no_meta_corpus_dir"; then
  echo "runner unexpectedly accepted a corpus without metadata" >&2
  exit 1
fi
jq -e '
  .status == "failed" and
  .failure.stage == "corpus-validation" and
  .corpus.meta == null
' "$no_meta_results/manifest.json" >/dev/null

invalid_rates_results=$(new_results_dir invalid-rates)
for result in manifest single concurrency throughput; do
  printf 'stale\n' > "$invalid_rates_results/$result.json"
done
printf 'preserve\n' > "$invalid_rates_results/unrelated.txt"
if run_runner "$success_port" "$invalid_rates_results" "$FIXTURE_DIR" RATES=1e3; then
  echo "runner unexpectedly accepted a non-decimal rate" >&2
  exit 1
fi
jq -e '.status == "failed" and .failure.stage == "configuration"' \
  "$invalid_rates_results/manifest.json" >/dev/null
for result in single concurrency throughput; do
  if [ -e "$invalid_rates_results/$result.json" ]; then
    echo "runner left a stale $result result" >&2
    exit 1
  fi
done
test "$(cat "$invalid_rates_results/unrelated.txt")" = preserve

invalid_iterations_results=$(new_results_dir invalid-iterations)
if run_runner "$success_port" "$invalid_iterations_results" "$FIXTURE_DIR" ITERATIONS=0; then
  echo "runner unexpectedly accepted a non-positive iteration count" >&2
  exit 1
fi
jq -e '.status == "failed" and .failure.stage == "configuration"' \
  "$invalid_iterations_results/manifest.json" >/dev/null

nonnumeric_iterations_results=$(new_results_dir nonnumeric-iterations)
if run_runner "$success_port" "$nonnumeric_iterations_results" "$FIXTURE_DIR" \
  ITERATIONS=invalid; then
  echo "runner unexpectedly accepted a nonnumeric iteration count" >&2
  exit 1
fi
jq -e '
  .status == "failed" and
  .failure.stage == "configuration" and
  .scenarios.single.iterations == null
' "$nonnumeric_iterations_results/manifest.json" >/dev/null

invalid_timeout_results=$(new_results_dir invalid-timeout)
if run_runner "$success_port" "$invalid_timeout_results" "$FIXTURE_DIR" READY_TIMEOUT=30x; then
  echo "runner unexpectedly accepted an invalid readiness timeout" >&2
  exit 1
fi
jq -e '.status == "failed" and .failure.stage == "configuration"' \
  "$invalid_timeout_results/manifest.json" >/dev/null

signal_results=$(new_results_dir signal)
signal_container="juno-rpc-benchmark-signal-$$"
container_names="$container_names $signal_container"
docker run -d --name "$signal_container" --network host --user 12345:12345 \
  --entrypoint "$RUNNER" \
  -v "$FIXTURE_DIR:/corpus:ro" \
  -v "$signal_results:/results" \
  -e NODE_URL="http://127.0.0.1:$success_port/v0_10" \
  -e READY_URL="http://127.0.0.1:$success_port/ready/rpc" \
  -e CORPUS_PATH=/corpus/corpus.json \
  -e EXPECTED_CHAIN_ID=0x534e5f4d41494e \
  -e EXPECTED_BLOCK_NUMBER=800000 \
  -e RUN_ID=signal-test \
  -e SNAPSHOT_ID=test-snapshot \
  -e SNAPSHOT_SHA256=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  -e JUNO_IMAGE_DIGEST=sha256:juno \
  -e RUNNER_IMAGE_DIGEST=sha256:runner \
  -e ITERATIONS=2 \
  -e VUS=1 \
  -e CONCURRENCY_DURATION=30s \
  -e THROUGHPUT_DURATION=1s \
  -e RATES=10 \
  -e THROUGHPUT_VUS=3 \
  "$IMAGE" >/dev/null

attempts=0
until jq -e '.scenarios.concurrency.status == "running"' "$signal_results/manifest.json" >/dev/null 2>&1; do
  attempts=$((attempts + 1))
  if [ "$attempts" -ge 100 ] || [ "$(docker inspect -f '{{.State.Running}}' "$signal_container")" != true ]; then
    docker logs "$signal_container" >&2 || true
    echo "runner did not reach the concurrency stage" >&2
    exit 1
  fi
  sleep 0.1
done
docker stop --time 5 "$signal_container" >/dev/null
test "$(docker inspect -f '{{.State.ExitCode}}' "$signal_container")" = 143
jq -e '
  .status == "failed" and
  .finishedAt != null and
  .failure.stage == "concurrency" and
  .failure.reason == "runner terminated by TERM" and
  .scenarios.concurrency.status == "failed"
' "$signal_results/manifest.json" >/dev/null

start_stub not-ready 0
not_ready_port=$stub_port
timeout_results=$(new_results_dir timeout)
if run_runner "$not_ready_port" "$timeout_results" "$FIXTURE_DIR" READY_TIMEOUT=1s; then
  echo "runner unexpectedly accepted a node that was not ready" >&2
  exit 1
fi
jq -e '.status == "failed" and .failure.stage == "readiness"' \
  "$timeout_results/manifest.json" >/dev/null

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

echo "benchmark runner integration tests passed for $RUNNER"

#!/bin/sh

set -eu
if (set -o pipefail) 2>/dev/null; then
  set -o pipefail
fi

readonly SCRIPT_DIR=/bench/rpc
readonly JUNO_COMMIT_FILE="$SCRIPT_DIR/juno-commit"
readonly WARMUP_ITERATIONS=200
CORPUS_PATH=${CORPUS_PATH:-$SCRIPT_DIR/corpus/v0_10/getTransactionByHash.json}
readonly CORPUS_PATH

if ! JUNO_COMMIT=$(cat "$JUNO_COMMIT_FILE"); then
  echo "embedded Juno commit is unavailable" >&2
  exit 2
fi
if ! printf '%s\n' "$JUNO_COMMIT" | grep -Eq '^[0-9a-f]{40}$'; then
  echo "embedded Juno commit is invalid" >&2
  exit 2
fi
readonly JUNO_COMMIT

required_env="
NODE_URL
READY_URL
EXPECTED_CHAIN_ID
EXPECTED_BLOCK_NUMBER
SNAPSHOT_ID
SNAPSHOT_SHA256
JUNO_IMAGE_DIGEST
RUNNER_IMAGE_DIGEST
"

for name in $required_env; do
  eval "value=\${$name-}"
  if [ -z "$value" ]; then
    echo "$name is required" >&2
    exit 2
  fi
done

RUN_ID=${RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$(printf '%.12s' "$JUNO_COMMIT")}
readonly RUN_ID
RESULTS_DIR=${RESULTS_DIR:-/results}
READY_TIMEOUT=${READY_TIMEOUT:-30m}
READY_POLL_INTERVAL=${READY_POLL_INTERVAL:-5s}
ITERATIONS_VALUE=${ITERATIONS:-200}
VUS_VALUE=${VUS:-50}
CONCURRENCY_DURATION_VALUE=${CONCURRENCY_DURATION:-30s}
THROUGHPUT_DURATION_VALUE=${THROUGHPUT_DURATION:-5s}
THROUGHPUT_VUS_VALUE=${THROUGHPUT_VUS:-50}
RATES_VALUE=${RATES:-1000,2000,3000}

started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
finished_at=
run_status=running
current_stage=preflight
failure_reason=
# Pre-flight and configuration rejections exit 2; readiness onwards exits 1.
fail_exit_code=2
ready_status=pending
warmup_status=pending
single_status=pending
concurrency_status=pending
throughput_status=pending
actual_chain_id=
actual_block_number=
actual_juno_version=
actual_corpus_sha=
corpus_meta=null
rates_json='[]'
warmup_metrics=null
single_metrics=null
concurrency_metrics=null
throughput_metrics=null
active_pid=

write_manifest() {
  manifest_tmp="$RESULTS_DIR/.manifest.json.tmp.$$"
  jq -n \
    --arg runId "$RUN_ID" \
    --arg status "$run_status" \
    --arg startedAt "$started_at" \
    --arg finishedAt "$finished_at" \
    --arg failedStage "$current_stage" \
    --arg failureReason "$failure_reason" \
    --arg nodeUrl "$NODE_URL" \
    --arg expectedChainId "$EXPECTED_CHAIN_ID" \
    --arg actualChainId "$actual_chain_id" \
    --arg expectedBlockNumber "$EXPECTED_BLOCK_NUMBER" \
    --arg actualBlockNumber "$actual_block_number" \
    --arg readyStatus "$ready_status" \
    --arg snapshotId "$SNAPSHOT_ID" \
    --arg snapshotSha256 "$SNAPSHOT_SHA256" \
    --arg corpusSha256 "$actual_corpus_sha" \
    --argjson corpusMeta "$corpus_meta" \
    --arg junoCommit "$JUNO_COMMIT" \
    --arg junoVersion "$actual_juno_version" \
    --arg junoImageDigest "$JUNO_IMAGE_DIGEST" \
    --arg runnerImageDigest "$RUNNER_IMAGE_DIGEST" \
    --arg warmupStatus "$warmup_status" \
    --arg singleStatus "$single_status" \
    --arg concurrencyStatus "$concurrency_status" \
    --arg throughputStatus "$throughput_status" \
    --arg warmupIterations "$WARMUP_ITERATIONS" \
    --arg iterations "$ITERATIONS_VALUE" \
    --arg vus "$VUS_VALUE" \
    --arg concurrencyDuration "$CONCURRENCY_DURATION_VALUE" \
    --arg throughputDuration "$THROUGHPUT_DURATION_VALUE" \
    --arg throughputVus "$THROUGHPUT_VUS_VALUE" \
    --argjson rates "$rates_json" \
    --argjson warmupMetrics "$warmup_metrics" \
    --argjson singleMetrics "$single_metrics" \
    --argjson concurrencyMetrics "$concurrency_metrics" \
    --argjson throughputMetrics "$throughput_metrics" \
    '
      # Parsed defensively: block numbers are validated after they are recorded.
      def as_number: try tonumber catch null;
      {
        schemaVersion: 2,
        runId: $runId,
        status: $status,
        startedAt: $startedAt,
        finishedAt: (if $finishedAt == "" then null else $finishedAt end),
        failure: (if $failureReason == "" then null else {
          stage: $failedStage,
          reason: $failureReason
        } end),
        node: {
          url: $nodeUrl,
          readiness: $readyStatus,
          expectedChainId: $expectedChainId,
          chainId: (if $actualChainId == "" then null else $actualChainId end),
          expectedBlockNumber: ($expectedBlockNumber | as_number),
          blockNumber: (if $actualBlockNumber == "" then null else ($actualBlockNumber | as_number) end)
        },
        snapshot: { id: $snapshotId, sha256: $snapshotSha256 },
        corpus: {
          sha256: (if $corpusSha256 == "" then null else $corpusSha256 end),
          meta: $corpusMeta
        },
        juno: {
          commit: $junoCommit,
          version: (if $junoVersion == "" then null else $junoVersion end),
          imageDigest: $junoImageDigest
        },
        runner: { imageDigest: $runnerImageDigest },
        scenarios: {
          warmup: {
            status: $warmupStatus,
            measured: false,
            iterations: ($warmupIterations | tonumber),
            metrics: $warmupMetrics
          },
          single: {
            status: $singleStatus,
            measured: true,
            iterations: ($iterations | as_number),
            metrics: $singleMetrics,
            result: "single.json"
          },
          concurrency: {
            status: $concurrencyStatus,
            measured: true,
            vus: ($vus | as_number),
            duration: $concurrencyDuration,
            metrics: $concurrencyMetrics,
            result: "concurrency.json"
          },
          throughput: {
            status: $throughputStatus,
            measured: true,
            preAllocatedVUs: ($throughputVus | as_number),
            maxVUs: ($throughputVus | as_number),
            rates: $rates,
            duration: $throughputDuration,
            metrics: $throughputMetrics,
            result: "throughput.json"
          }
        }
      }
    ' > "$manifest_tmp"
  mv "$manifest_tmp" "$RESULTS_DIR/manifest.json"
}

fail() {
  failure_reason=$*
  run_status=failed
  finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  echo "benchmark runner failed during $current_stage: $failure_reason" >&2
  write_manifest
  trap - EXIT
  exit "$fail_exit_code"
}

set_scenario_status() {
  eval "$1_status=\$2"
}

mark_current_stage_failed() {
  case "$current_stage" in
    readiness) ready_status=failed ;;
    warmup|single|concurrency|throughput) set_scenario_status "$current_stage" failed ;;
  esac
}

on_signal() {
  signal=$1
  exit_code=$2
  trap - TERM INT EXIT
  if [ -n "$active_pid" ]; then
    kill -"$signal" "$active_pid" 2>/dev/null || true
    wait "$active_pid" 2>/dev/null || true
    active_pid=
  fi
  mark_current_stage_failed
  run_status=failed
  failure_reason="runner terminated by $signal"
  finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  set +e
  write_manifest
  echo "benchmark runner terminated by $signal during $current_stage" >&2
  exit "$exit_code"
}

run_tracked() {
  "$@" &
  active_pid=$!
  wait_for_active
}

run_tracked_input() {
  input=$1
  shift
  "$@" < "$input" &
  active_pid=$!
  wait_for_active
}

wait_for_active() {
  set +e
  wait "$active_pid"
  child_status=$?
  set -e
  active_pid=
  return "$child_status"
}

on_exit() {
  exit_code=$?
  trap - EXIT
  if [ "$exit_code" -ne 0 ] && [ "$run_status" != failed ]; then
    run_status=failed
    failure_reason="runner exited with status $exit_code"
    finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    set +e
    write_manifest
  fi
  exit "$exit_code"
}

mkdir -p "$RESULTS_DIR"
if [ -n "${RUN_ID_FILE:-}" ]; then
  run_id_tmp="${RUN_ID_FILE}.tmp.$$"
  printf '%s\n' "$RUN_ID" > "$run_id_tmp"
  mv "$run_id_tmp" "$RUN_ID_FILE"
fi
rm -f \
  "$RESULTS_DIR/manifest.json" \
  "$RESULTS_DIR/single.json" \
  "$RESULTS_DIR/concurrency.json" \
  "$RESULTS_DIR/throughput.json" \
  "$RESULTS_DIR"/.manifest.json.tmp.* \
  "$RESULTS_DIR"/.rpc.json.tmp.* \
  "$RESULTS_DIR"/.warmup.json.tmp.* \
  "$RESULTS_DIR"/.single.json.tmp.* \
  "$RESULTS_DIR"/.concurrency.json.tmp.* \
  "$RESULTS_DIR"/.throughput.json.tmp.*

trap on_exit EXIT
trap 'on_signal TERM 143' TERM
trap 'on_signal INT 130' INT

current_stage=corpus-validation
checksum_file="${CORPUS_PATH}.sha256"
if [ ! -r "$CORPUS_PATH" ]; then
  fail "corpus is not readable: $CORPUS_PATH"
fi
if [ ! -r "$checksum_file" ]; then
  fail "corpus checksum is not readable: $checksum_file"
fi

expected_corpus_sha=$(awk 'NR == 1 { print $1 }' "$checksum_file")
actual_corpus_sha=$(sha256sum "$CORPUS_PATH" | awk '{ print $1 }')
if [ -z "$expected_corpus_sha" ] || [ "$actual_corpus_sha" != "$expected_corpus_sha" ]; then
  fail "corpus checksum mismatch"
fi

corpus_meta=$(jq -ce '.meta | objects' "$CORPUS_PATH") || {
  corpus_meta=null
  fail "corpus metadata is invalid"
}
jq -e '.requests | arrays | length > 0' "$CORPUS_PATH" >/dev/null || {
  fail "corpus requests are invalid or empty"
}

current_stage=configuration
rates_json=$(printf '%s' "$RATES_VALUE" | jq -Rce '
  split(",")
  | if length > 0 and all(.[]; test("^\\s*[0-9]+\\s*$")) then
      map(gsub("\\s"; "") | tonumber)
    else
      error("invalid rate")
    end
') || {
  rates_json='[]'
  fail "RATES must be a comma-separated list of integers"
}
if ! printf '%s' "$rates_json" | jq -e 'all(.[]; . > 0 and . <= 2147483647 and floor == .)' >/dev/null; then
  fail "RATES must contain positive 32-bit integers"
fi
RATES_VALUE=$(printf '%s' "$rates_json" | jq -r 'join(",")')

require_positive_integer() {
  case "$2" in
    ''|*[!0-9]*) fail "$1 must be a positive integer" ;;
  esac
  if [ "$2" -le 0 ]; then
    fail "$1 must be a positive integer"
  fi
}

require_positive_integer ITERATIONS "$ITERATIONS_VALUE"
require_positive_integer VUS "$VUS_VALUE"
require_positive_integer THROUGHPUT_VUS "$THROUGHPUT_VUS_VALUE"

case "$EXPECTED_BLOCK_NUMBER" in
  ''|*[!0-9]*)
    fail "EXPECTED_BLOCK_NUMBER must be a non-negative integer"
    ;;
esac

duration_seconds() {
  raw=$1
  case "$raw" in
    *s) value=${raw%s}; multiplier=1 ;;
    *m) value=${raw%m}; multiplier=60 ;;
    *h) value=${raw%h}; multiplier=3600 ;;
    *) value=$raw; multiplier=1 ;;
  esac
  case "$value" in
    ''|*[!0-9]*) return 1 ;;
  esac
  [ "$value" -gt 0 ] || return 1
  echo $((value * multiplier))
}

timeout_seconds=$(duration_seconds "$READY_TIMEOUT") || fail "READY_TIMEOUT must be a positive duration using s, m, or h"
write_manifest

fail_exit_code=1
current_stage=readiness
ready_status=waiting
write_manifest
ready_started=$(date +%s)
while ! run_tracked curl --fail --silent --show-error --max-time 5 "$READY_URL" >/dev/null; do
  now=$(date +%s)
  if [ $((now - ready_started)) -ge "$timeout_seconds" ]; then
    ready_status=failed
    fail "RPC readiness timed out after $READY_TIMEOUT"
  fi
  echo "waiting for RPC readiness at $READY_URL" >&2
  run_tracked sleep "$READY_POLL_INTERVAL"
done
ready_status=passed
write_manifest

rpc_result() {
  method=$1
  rpc_tmp="$RESULTS_DIR/.rpc.json.tmp.$$"
  if ! run_tracked curl --fail --silent --show-error --max-time 10 \
    -H 'Content-Type: application/json' \
    --data "{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"params\":[],\"id\":1}" \
    "$NODE_URL" > "$rpc_tmp"; then
    rm -f "$rpc_tmp"
    return 1
  fi
  if ! rpc_value=$(jq -er 'if .error then error(.error.message // "JSON-RPC error") elif has("result") then .result else error("missing JSON-RPC result") end' "$rpc_tmp"); then
    rm -f "$rpc_tmp"
    return 1
  fi
  rm -f "$rpc_tmp"
}

extract_summary_metrics() {
  jq -ce -f "$SCRIPT_DIR/summary-metrics.jq" "$1"
}

current_stage=target-validation
rpc_result juno_version || fail "could not read Juno version"
actual_juno_version=$rpc_value
expected_juno_version="sha-${JUNO_COMMIT}"
if [ "$actual_juno_version" != "$expected_juno_version" ]; then
  fail "Juno image mismatch: expected $expected_juno_version, got $actual_juno_version"
fi
rpc_result starknet_chainId || fail "could not read chain ID"
actual_chain_id=$rpc_value
if [ "$actual_chain_id" != "$EXPECTED_CHAIN_ID" ]; then
  fail "chain ID mismatch: expected $EXPECTED_CHAIN_ID, got $actual_chain_id"
fi
rpc_result starknet_blockNumber || fail "could not read block number"
actual_block_number=$rpc_value
case "$actual_block_number" in
  ''|*[!0-9]*) fail "node returned an invalid block number: $actual_block_number" ;;
esac
if [ "$actual_block_number" != "$EXPECTED_BLOCK_NUMBER" ]; then
  fail "block number mismatch: expected $EXPECTED_BLOCK_NUMBER, got $actual_block_number"
fi
write_manifest

current_stage=warmup
warmup_status=running
write_manifest
warmup_tmp="$RESULTS_DIR/.warmup.json.tmp.$$"
warmup_exit=0
run_tracked_input "$CORPUS_PATH" k6 run --quiet \
  -e NODE_URL="$NODE_URL" \
  --vus 1 \
  --iterations "$WARMUP_ITERATIONS" \
  --summary-export "$warmup_tmp" \
  "$SCRIPT_DIR/run.js" > /dev/null || warmup_exit=$?
if ! warmup_metrics=$(extract_summary_metrics "$warmup_tmp"); then
  rm -f "$warmup_tmp"
  warmup_metrics=null
  warmup_status=failed
  fail "warmup did not produce a valid summary"
fi
rm -f "$warmup_tmp"
warmup_failed_checks=$(printf '%s' "$warmup_metrics" | jq -r '.failedChecks')
warmup_request_failures=$(printf '%s' "$warmup_metrics" | jq -r '.requestFailures')
warmup_vu_failures=$(printf '%s' "$warmup_metrics" | jq -r '.vuFailures')
if [ "$warmup_exit" -ne 0 ] || [ "$warmup_failed_checks" -ne 0 ] || \
  [ "$warmup_request_failures" -ne 0 ] || [ "$warmup_vu_failures" -ne 0 ]; then
  warmup_status=failed
  fail "warmup recorded check, request, or VU failures"
fi
warmup_status=passed
write_manifest

run_scenario() {
  scenario=$1
  result_tmp="$RESULTS_DIR/.${scenario}.json.tmp.$$"
  result_file="$RESULTS_DIR/${scenario}.json"

  set_scenario_status "$scenario" running
  current_stage=$scenario
  write_manifest

  scenario_exit=0
  case "$scenario" in
    single)
      run_tracked_input "$CORPUS_PATH" k6 run --quiet \
        -e NODE_URL="$NODE_URL" \
        --vus 1 \
        --iterations "$ITERATIONS_VALUE" \
        --summary-export "$result_tmp" \
        --summary-trend-stats 'avg,min,med,p(90),p(99),max' \
        "$SCRIPT_DIR/run.js" || scenario_exit=$?
      ;;
    concurrency)
      run_tracked_input "$CORPUS_PATH" k6 run --quiet \
        -e NODE_URL="$NODE_URL" \
        --vus "$VUS_VALUE" \
        --duration "$CONCURRENCY_DURATION_VALUE" \
        --summary-export "$result_tmp" \
        --summary-trend-stats 'avg,min,med,p(90),p(99),max' \
        "$SCRIPT_DIR/run.js" || scenario_exit=$?
      ;;
    throughput)
      run_tracked_input "$CORPUS_PATH" k6 run --quiet \
        -e NODE_URL="$NODE_URL" \
        -e RATES="$RATES_VALUE" \
        -e DURATION="$THROUGHPUT_DURATION_VALUE" \
        -e THROUGHPUT_VUS="$THROUGHPUT_VUS_VALUE" \
        --summary-export "$result_tmp" \
        --summary-trend-stats 'avg,min,med,p(90),p(99),max' \
        "$SCRIPT_DIR/throughput.js" || scenario_exit=$?
      ;;
  esac

  if ! scenario_metrics=$(extract_summary_metrics "$result_tmp"); then
    if [ -s "$result_tmp" ]; then
      mv "$result_tmp" "$result_file"
    else
      rm -f "$result_tmp"
    fi
    set_scenario_status "$scenario" failed
    fail "$scenario did not produce a valid summary"
  fi
  mv "$result_tmp" "$result_file"

  eval "${scenario}_metrics=\$scenario_metrics"

  failed_checks=$(printf '%s' "$scenario_metrics" | jq -r '.failedChecks')
  request_failures=$(printf '%s' "$scenario_metrics" | jq -r '.requestFailures')
  vu_failures=$(printf '%s' "$scenario_metrics" | jq -r '.vuFailures')
  dropped_iterations=$(printf '%s' "$scenario_metrics" | jq -r '.droppedIterations')

  if [ "$scenario_exit" -ne 0 ]; then
    set_scenario_status "$scenario" failed
    fail "$scenario exited with status $scenario_exit"
  fi

  if [ "$failed_checks" -ne 0 ] || [ "$request_failures" -ne 0 ] || \
    [ "$vu_failures" -ne 0 ]; then
    set_scenario_status "$scenario" failed
    fail "$scenario recorded check, request, or VU failures"
  fi

  # Saturation is a measurement, not a harness error; droppedIterations records it.
  if [ "$dropped_iterations" -ne 0 ]; then
    echo "$scenario dropped $dropped_iterations scheduled iterations: offered load exceeded the worker pool" >&2
  fi

  set_scenario_status "$scenario" passed
  write_manifest
}

run_scenario single
run_scenario concurrency
run_scenario throughput

current_stage=complete
run_status=passed
finished_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
write_manifest
echo "benchmark run $RUN_ID completed successfully" >&2

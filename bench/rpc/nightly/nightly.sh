#!/usr/bin/env bash
# Stable entrypoint for the managed nightly RPC benchmark.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RPC_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CATALOG="$RPC_DIR/corpus/nightly.json"
CI_CONFIG="$SCRIPT_DIR/ci.json"
CORPUS_GEN=${CORPUS_GEN:-"$RPC_DIR/../../build/corpus-gen"}
RUN_SCRIPT="$RPC_DIR/run.js"

: "${NODE_URL:?NODE_URL is required}"
: "${SNAPSHOT_SHA256:?SNAPSHOT_SHA256 is required}"
[[ $SNAPSHOT_SHA256 =~ ^[0-9a-f]{64}$ ]] || {
  echo "error: SNAPSHOT_SHA256 must be a lowercase SHA-256 digest" >&2
  exit 1
}
CORPUS_ROOT="${CORPUS_ROOT:-/corpus/cases}/$SNAPSHOT_SHA256"

source "$SCRIPT_DIR/internal/preflight.sh"
source "$SCRIPT_DIR/internal/corpus.sh"

case_path() {
  printf '%s/%s/corpus.json\n' "$CORPUS_ROOT" "$1"
}

benchmark() {
  [[ -f $CI_CONFIG ]] || {
    echo "error: k6 config not found: $CI_CONFIG" >&2
    return 1
  }
  local out_dir=${RESULTS_DIR:-/results}
  local warmup_iterations=${WARMUP_ITERATIONS:-200}
  mkdir -p "$out_dir"

  local name corpus summary case_failed
  local -a failed=()
  while IFS= read -r name; do
    summary="$out_dir/$name.json"

    echo
    echo "==> $name"
    corpus=$(case_path "$name")
    if [[ ! -f $corpus ]]; then
      echo "error: missing corpus for $name: $corpus" >&2
      failed+=("$name")
      continue
    fi
    corpus="$(cd "$(dirname "$corpus")" && pwd)/$(basename "$corpus")"

    case_failed=0
    if [[ $warmup_iterations != 0 ]]; then
      echo "warming up"
      (
        K6_WEB_DASHBOARD=false k6 run \
          --vus 1 \
          --iterations "$warmup_iterations" \
          --tag "case_id=$name" \
          --tag phase=warmup \
          -e NODE_URL="$NODE_URL" \
          "$RUN_SCRIPT" <"$corpus" >/dev/null
      ) || {
        echo "error: warmup did not complete for $name" >&2
        case_failed=1
      }
    fi

    if ((case_failed == 0)); then
      k6 run \
        --config "$CI_CONFIG" \
        --tag "case_id=$name" \
        --tag phase=measure \
        --summary-export "$summary" \
        -e NODE_URL="$NODE_URL" \
        "$RUN_SCRIPT" <"$corpus" || case_failed=1
    fi
    ((case_failed == 0)) || failed+=("$name")
  done < <(jq -r 'keys_unsorted[]' "$CATALOG")

  if ((${#failed[@]} > 0)); then
    echo "failed: ${failed[*]}" >&2
    return 1
  fi
}

nightly_run() {
  preflight

  local failed=0
  generate || failed=1
  benchmark || failed=1
  return "$failed"
}

nightly_run

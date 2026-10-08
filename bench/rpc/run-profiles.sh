#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/resolve-node.sh"

usage() {
  cat >&2 <<EOF
usage: $0 <node> <corpus> <plan.json> <trial> [profile...]
  <node>         started via nodes/<node>.sh, configured by nodes/<node>.json
                 merged with the gitignored nodes/<node>.local.json (local keys
                 win; see nodes/node.local.json.example):
                   url         JSON-RPC endpoint
                   snapshot    pristine DB directory, copied fresh for each profile
                               unless the plan sets restore to false
                   workspaces  each profile's workspace -> <workspaces>/<corpus>/<profile>/<trial>/
                   bin         node binary; nodes/<node>.sh gets it as NODE_BIN and
                               the profile's DB as NODE_DB
  <corpus>       expanded config or its folder, e.g. corpus/report.json
  <plan.json>    shared by all machines:
                   height       block height the node must report; null skips the check
                   drop_caches  before each profile; needs passwordless sudo; default true
                   restore      copy the snapshot for each profile; false runs the node
                                on the snapshot itself, which it may modify; default true
                   k6_flags     k6 flags for every profile, before its own;
                                e.g. "-e GZIP=1" for gzip responses
                   profiles     {"<profile>": "<k6 flags>"}, run in file order
  <trial>        name, e.g. 1
  [profile...]   profiles to run; default: all
A workspace must not exist; it gets db/ (only with restore; removed after
the run), node.log and the run-all.sh results.
EOF
  exit 1
}

fail() {
  echo "error: $*" >&2
  exit 1
}

required_field() {
  jq -er --arg key "$1" '.[$key] // empty | select(. != "")' <<<"$NODE_CFG" ||
    fail "node '$NODE' must set \"$1\" in $NODES_DIR/$NODE.local.json"
}

load_node() {
  NODE=$1
  resolve_node "$NODE"
  [[ -n $NODE_CFG ]] || fail "<node> must name a nodes/ config, not a URL: $NODE"

  SNAPSHOT=$(required_field snapshot)
  WORKSPACES=$(required_field workspaces)
  NODE_BIN=$(required_field bin)
  export NODE_BIN

  NODE_CTL="$NODES_DIR/$NODE.sh"
  [[ -x $NODE_CTL ]] || fail "$NODE_CTL not found"
  [[ -d $SNAPSHOT ]] || fail "$SNAPSHOT not found"
}

load_corpus() {
  CORPUS=$1
  [[ -e $CORPUS ]] || fail "$CORPUS not found"
  CORPUS_NAME=$(basename "${CORPUS%/}" .json)
}

# jq's // treats false as missing, and -e fails on a false output: hence the
# explicit null check and tostring.
plan_bool() {
  jq -er --arg key "$1" --argjson default "$2" \
    'if .[$key] == null then $default else .[$key] end | booleans | tostring' "$PLAN" ||
    fail "$PLAN: $1 must be true or false"
}

load_plan() {
  PLAN=$1
  [[ -f $PLAN ]] || fail "$PLAN not found"

  HEIGHT=$(jq -r '.height // empty' "$PLAN")
  if [[ -n $HEIGHT && ! $HEIGHT =~ ^[0-9]+$ ]]; then
    fail "$PLAN: height must be a block number or null"
  fi

  RESTORE=$(plan_bool restore true)
  DROP_CACHES=$(plan_bool drop_caches true)
  [[ $DROP_CACHES != true ]] || sudo -n true 2>/dev/null ||
    fail "drop_caches needs passwordless sudo; set \"drop_caches\": false in $PLAN to skip"
  K6_FLAGS=$(jq -er '.k6_flags // "" | strings' "$PLAN") || fail "$PLAN: k6_flags must be a string"
}

select_profiles() {
  if (($# > 0)); then
    PROFILES=("$@")
  else
    local profiles
    profiles=$(jq -er '.profiles | keys_unsorted[]' "$PLAN") ||
      fail "$PLAN: profiles must be a non-empty object"
    mapfile -t PROFILES <<<"$profiles"
  fi
}

profile_flags() {
  jq -er --arg profile "$1" '.profiles[$profile]' "$PLAN"
}

workspace_dir() {
  local profile=$1 trial=$2
  echo "$WORKSPACES/$CORPUS_NAME/$profile/$trial"
}

# A profile runs for hours: catch a bad one before the first starts.
check_profiles() {
  local trial=$1 profile workspace
  for profile in "${PROFILES[@]}"; do
    profile_flags "$profile" >/dev/null || fail "unknown profile '$profile'"
    workspace=$(workspace_dir "$profile" "$trial")
    [[ ! -e $workspace ]] || fail "$workspace already exists"
  done
}

prepare_db() {
  local db=$1
  if [[ $RESTORE != true ]]; then
    echo "==> using $SNAPSHOT in place"
    export NODE_DB="$SNAPSHOT"
    return
  fi
  echo "==> restore $db from $SNAPSHOT"
  export NODE_DB="$db"
  DB_COPY="$db"
  cp -a "$SNAPSHOT" "$db"
}

drop_caches() {
  if [[ $DROP_CACHES != true ]]; then
    echo "==> skipping drop_caches"
    return
  fi
  echo "==> dropping caches"
  sync
  echo 3 | sudo -n tee /proc/sys/vm/drop_caches >/dev/null
}

block_number_response() {
  curl --silent --max-time 5 --header 'Content-Type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"starknet_blockNumber","params":[]}' \
    "$NODE_URL"
}

check_url_free() {
  if block_number_response >/dev/null; then
    fail "$NODE_URL already answers; stop that node first"
  fi
}

node_alive() {
  kill -0 "$NODE_PID" 2>/dev/null
}

start_node() {
  local log=$1
  echo "==> starting $NODE (log: $log)"
  "$NODE_CTL" >"$log" 2>&1 &
  NODE_PID=$!
}

wait_for_node() {
  local log=$1 waited=0
  until block_number_response >/dev/null; do
    node_alive || fail "$NODE exited; see $log"
    sleep 2
    waited=$((waited + 2))
    if ((waited % 30 == 0)); then
      echo "waiting for $NODE (${waited}s)"
    fi
  done
}

check_height() {
  local log=$1 response height
  response=$(block_number_response)
  height=$(jq -r '.result // empty' <<<"$response")
  [[ $height =~ ^[0-9]+$ ]] || fail "$NODE answered without a block number: $response; see $log"
  echo "==> $NODE ready at $height"
  if [[ -n $HEIGHT ]] && ((height != HEIGHT)); then
    fail "node at $height, expected $HEIGHT"
  fi
}

stop_node() {
  echo "==> stopping $NODE"
  if node_alive; then
    kill -TERM "$NODE_PID"
  fi
  wait "$NODE_PID" || true
  unset NODE_PID
}

cleanup() {
  if [[ -n ${NODE_PID-} ]]; then
    stop_node
  fi
  if [[ -n ${DB_COPY-} ]]; then
    rm -rf "$DB_COPY"
    unset DB_COPY
  fi
}

run_k6() {
  local workspace=$1
  shift
  echo "==> k6 on $CORPUS: $*"
  OUT_DIR="$workspace" "$SCRIPT_DIR/run-all.sh" "$CORPUS" "$NODE" "$@"
}

run_profile() {
  local profile=$1 trial=$2 workspace log flags
  workspace=$(workspace_dir "$profile" "$trial")
  log="$workspace/node.log"
  read -ra flags <<<"$K6_FLAGS $(profile_flags "$profile")"

  echo
  echo "######## profile $profile (trial $trial)"
  check_url_free
  mkdir -p "$workspace"
  prepare_db "$workspace/db"
  drop_caches
  start_node "$log"
  wait_for_node "$log"
  check_height "$log"
  run_k6 "$workspace" "${flags[@]}" || FAILED+=("$profile")
  cleanup
  echo "==> results in $workspace"
}

main() {
  (($# >= 4)) || usage
  local node=$1 corpus=$2 plan=$3 trial=$4
  shift 4

  load_node "$node"
  load_corpus "$corpus"
  load_plan "$plan"
  select_profiles "$@"
  check_profiles "$trial"

  # cleanup deletes DB_COPY and kills NODE_PID: never act on inherited ones.
  unset NODE_PID DB_COPY
  trap cleanup EXIT

  FAILED=()
  local profile
  for profile in "${PROFILES[@]}"; do
    run_profile "$profile" "$trial"
  done
  ((${#FAILED[@]} == 0)) || fail "k6 failed for profiles: ${FAILED[*]}"
}

main "$@"

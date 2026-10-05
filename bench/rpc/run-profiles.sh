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
                   workspaces  each profile's workspace -> <workspaces>/<corpus>/<profile>/<trial>/
                   bin         node binary; nodes/<node>.sh gets it as NODE_BIN and
                               the profile's DB copy as NODE_DB
  <corpus>       expanded config or its folder, e.g. corpus/report.json
  <plan.json>    shared by all machines:
                   height       block height the node must report; null skips the check
                   drop_caches  before each profile; needs root; default true
                   k6_flags     k6 flags for every profile, before its own;
                                e.g. "-e GZIP=1" for gzip responses
                   profiles     {"<profile>": "<k6 flags>"}, run in file order
  <trial>        name, e.g. 1
  [profile...]   profiles to run; default: all
A workspace must not exist; it gets db/ (removed after the run), node.log
and the run-all.sh results.
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

load_plan() {
  PLAN=$1
  [[ -f $PLAN ]] || fail "$PLAN not found"

  HEIGHT=$(jq -r '.height // empty' "$PLAN")
  if [[ -n $HEIGHT && ! $HEIGHT =~ ^[0-9]+$ ]]; then
    fail "$PLAN: height must be a block number or null"
  fi

  DROP_CACHES=$(jq '.drop_caches != false' "$PLAN")
  K6_FLAGS=$(jq -er '.k6_flags // "" | strings' "$PLAN") || fail "$PLAN: k6_flags must be a string"
}

select_profiles() {
  if (($# > 0)); then
    PROFILES=("$@")
  else
    mapfile -t PROFILES < <(jq -r '.profiles | keys_unsorted[]' "$PLAN")
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

restore_db() {
  local db=$1
  echo "==> restore $db from $SNAPSHOT"
  cp -a "$SNAPSHOT" "$db"
  export NODE_DB="$db"
}

drop_caches() {
  if [[ $DROP_CACHES != true ]]; then
    echo "==> skipping drop_caches"
    return
  fi
  echo "==> dropping caches"
  sync
  echo 3 >/proc/sys/vm/drop_caches
}

block_number() {
  curl --silent --max-time 5 --header 'Content-Type: application/json' \
    --data '{"jsonrpc":"2.0","id":1,"method":"starknet_blockNumber","params":[]}' \
    "$NODE_URL" | jq -r '.result // empty'
}

start_node() {
  local log=$1
  echo "==> starting $NODE (log: $log)"
  "$NODE_CTL" >"$log" 2>&1 &
  NODE_PID=$!
}

wait_for_node() {
  local waited=0
  until [[ -n $(block_number) ]]; do
    sleep 2
    waited=$((waited + 2))
    if ((waited % 30 == 0)); then
      echo "waiting for $NODE (${waited}s)"
    fi
  done
}

check_height() {
  local height
  height=$(block_number)
  echo "==> $NODE ready at $height"
  if [[ -n $HEIGHT ]] && ((height != HEIGHT)); then
    stop_node
    fail "node at $height, expected $HEIGHT"
  fi
}

stop_node() {
  echo "==> stopping $NODE"
  kill -TERM "$NODE_PID"
  wait "$NODE_PID" || true
}

run_k6() {
  local workspace=$1
  shift
  echo "==> k6 on $CORPUS: $*"
  # run-all.sh fails if any corpus failed; go on so the node still stops.
  OUT_DIR="$workspace" "$SCRIPT_DIR/run-all.sh" "$CORPUS" "$NODE" "$@" || true
}

run_profile() {
  local profile=$1 trial=$2 workspace flags
  workspace=$(workspace_dir "$profile" "$trial")
  read -ra flags <<<"$K6_FLAGS $(profile_flags "$profile")"

  echo
  echo "######## profile $profile (trial $trial)"
  mkdir -p "$workspace"
  restore_db "$workspace/db"
  drop_caches
  start_node "$workspace/node.log"
  wait_for_node
  check_height
  run_k6 "$workspace" "${flags[@]}"
  stop_node
  rm -rf "$workspace/db"
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

  local profile
  for profile in "${PROFILES[@]}"; do
    run_profile "$profile" "$trial"
  done
}

main "$@"

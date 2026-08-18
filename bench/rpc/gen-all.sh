#!/usr/bin/env bash
# Generate a corpus for every entry in a JSON config into the config's folder.
set -euo pipefail

usage() {
  cat >&2 <<EOF
usage: $0 <corpus(.json)> <node|url> [corpus-gen flags...]
  <corpus>  config or its folder (all.json <-> all/)
  <node>    nodes.json name or URL, sampled via --source-url
Config: {"name": "subcommand [flags]"} map, one corpus per entry.
Extra args pass to every corpus-gen call; per-entry flags win.
EOF
  exit 1
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORPUS_GEN="$SCRIPT_DIR/../../build/corpus-gen"
source "$SCRIPT_DIR/resolve-node.sh"

[[ $# -lt 2 ]] && usage
BASE=${1%/}
BASE=${BASE%.json}
CONFIG="$BASE.json"
OUT_DIR="$BASE"
resolve_node "$2"
shift 2
GEN_ARGS=("$@")

if [[ ! -f "$CONFIG" ]]; then
  echo "error: config $CONFIG not found" >&2
  exit 1
fi
if ! jq -e 'type == "object" and length > 0 and all(.[]; type == "string")' "$CONFIG" >/dev/null; then
  echo "error: $CONFIG must be a non-empty JSON object of string values" >&2
  exit 1
fi
if [[ ! -x "$CORPUS_GEN" ]]; then
  echo "error: $CORPUS_GEN not found; run 'make corpus-gen' first" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"

gen() {
  local name=$1
  shift
  echo "==> $name"
  # Node URL first, then pass-through, then per-entry flags (pflag: last wins).
  "$CORPUS_GEN" --source-url "$NODE_URL" "${GEN_ARGS[@]}" "$@" >"$OUT_DIR/$name.json" ||
    { rm -f "$OUT_DIR/$name.json"; return 1; }
}

while IFS=$'\t' read -r name cmd; do
  read -ra sub <<<"$cmd"
  gen "$name" "${sub[@]}"
done < <(jq -r 'to_entries[] | "\(.key)\t\(.value)"' "$CONFIG")

echo "corpora written to $OUT_DIR"

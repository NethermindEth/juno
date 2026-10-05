#!/usr/bin/env bash
# Replay every corpus in a folder with k6, one run per corpus.
set -euo pipefail

usage() {
  cat >&2 <<EOF
usage: $0 <corpus(.json)> <node|url> [k6 flags...]
  <corpus>  config or its folder (all.json <-> all/)
  <node>    nodes/<node>.json name or URL; results -> <corpus>/<node>/
Runs in config order when the config exists, else A-Z.
Extra args pass to every k6 run.
EOF
  exit 1
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/resolve-node.sh"

[[ $# -lt 2 ]] && usage
BASE=${1%/}
CORPUS_DIR=${BASE%.json}
CONFIG="$CORPUS_DIR.json"
resolve_node "$2"
shift 2
K6_ARGS=("$@")

OUT_DIR="$CORPUS_DIR/$NODE_NAME"

# Config order first (keys_unsorted keeps file order), then corpora the
# config doesn't list, A-Z.
shopt -s nullglob
mapfile -t names < <(
  {
    [[ -f "$CONFIG" ]] && jq -r 'keys_unsorted[]' "$CONFIG"
    for corpus in "$CORPUS_DIR"/*.json; do basename "$corpus" .json; done
  } | awk '!seen[$0]++'
)
if ((${#names[@]} == 0)); then
  echo "error: no corpora in $CORPUS_DIR" >&2
  exit 1
fi

corpora=()
missing=()
for name in "${names[@]}"; do
  if [[ -f "$CORPUS_DIR/$name.json" ]]; then
    corpora+=("$name")
  else
    missing+=("$name")
  fi
done
echo "run order:"
for i in "${!corpora[@]}"; do
  printf '  %2d. %s\n' "$((i + 1))" "${corpora[i]}"
done
if ((${#missing[@]} > 0)); then
  echo "missing: ${missing[*]}" >&2
fi
mkdir -p "$OUT_DIR"

failed=("${missing[@]}")
for name in "${corpora[@]}"; do
  corpus="$CORPUS_DIR/$name.json"
  echo
  echo "==> $name"
  # 2s aggregation period: the default 10s leaves short runs with too few
  # data points and k6 then skips the HTML report entirely.
  K6_WEB_DASHBOARD=true \
    K6_WEB_DASHBOARD_EXPORT="$OUT_DIR/$name.html" \
    K6_WEB_DASHBOARD_PERIOD="${K6_WEB_DASHBOARD_PERIOD:-2s}" \
    k6 run "$SCRIPT_DIR/run.js" \
    -e NODE_URL="$NODE_URL" \
    --summary-export "$OUT_DIR/$name.json" \
    --summary-trend-stats "avg,min,med,p(90),p(95),p(99),max" \
    "${K6_ARGS[@]}" \
    <"$corpus" ||
    failed+=("$name")
done

report="$OUT_DIR/report.md"
{
  echo "| method | reqs | req/s | errors | avg (ms) | med | p90 | p95 | p99 | max |"
  echo "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |"
  for name in "${corpora[@]}"; do
    summary="$OUT_DIR/$name.json"
    [[ -f "$summary" ]] || continue
    jq -r --arg name "$name" '
      def r2: (. // 0) * 100 | round / 100;
      .metrics as $m
      | ($m.http_req_duration // {}) as $d
      | (($m.checks.passes // 0) + ($m.checks.fails // 0)) as $checks
      | [ $name,
          ($m.http_reqs.count // 0),
          ($m.http_reqs.rate | r2),
          (if $checks > 0
           then "\($m.checks.fails / $checks * 100 | . * 100 | round / 100)%"
           else "n/a" end),
          ($d.avg | r2), ($d.med | r2), ($d["p(90)"] | r2),
          ($d["p(95)"] | r2), ($d["p(99)"] | r2), ($d.max | r2) ]
      | "| " + (map(tostring) | join(" | ")) + " |"
    ' "$summary"
  done
} >"$report"

echo
cat "$report"
echo
echo "results in $OUT_DIR"

if ((${#failed[@]} > 0)); then
  echo "failed: ${failed[*]}" >&2
  exit 1
fi

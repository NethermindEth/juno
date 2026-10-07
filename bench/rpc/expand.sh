#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<EOF
usage: $0 <sets> <input.json >output.json
  <sets>  measured sets per case (>= 1)
Each case becomes <case>.warmup (--seed=0), then <case>.1 .. <case>.<sets>
(--seed=1 .. --seed=<sets>), in input order.
EOF
  exit 1
}

[[ $# -eq 1 && $1 =~ ^[1-9][0-9]*$ ]] || usage

jq --argjson sets "$1" '
  to_entries
  | map(
      . as $case
      | (["warmup"] + [range(1; $sets + 1) | tostring])
      | to_entries[]
      | {key: "\($case.key).\(.value)", value: "\($case.value) --seed=\(.key)"}
    )
  | from_entries
'

#!/bin/sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/../../../.." && pwd)
catalog=${repo_root}/bench/rpc/corpus/nightly.json

# Validate filesystem-safe case definitions.
jq -e '
  type == "object" and length > 0 and
  all(to_entries[];
    (.key | test("^[A-Za-z0-9][A-Za-z0-9._-]*$")) and
    (.value | type == "string" and test("[^[:space:]]")))
' "${catalog}" >/dev/null

# Reject duplicate case IDs.
test -z "$(jq --stream -r '
  select(length == 2 and (.[0] | length) == 1) | .[0][0]
' "${catalog}" | sort | uniq -d)"

echo "nightly catalog validation passed"

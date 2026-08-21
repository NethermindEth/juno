generate_case() (
  set -euo pipefail
  local name=$1 command=$2 work case_dir
  local -a sub

  case_dir="$CORPUS_ROOT/$name"
  work=$(mktemp -d "$CORPUS_ROOT/.staging/$name.XXXXXX")
  trap 'rm -rf "$work"' EXIT INT TERM

  read -ra sub <<<"$command"
  "$CORPUS_GEN" --source-url "$NODE_URL" \
    --concurrency="${CONCURRENCY:-32}" \
    "${sub[@]}" >"$work/corpus.json" || return 1
  printf '%s\n' "$command" >"$work/command"
  [[ ! -e $case_dir ]] || {
    echo "error: case appeared during generation: $name" >&2
    return 1
  }
  mv -T "$work" "$case_dir"
  work=
  trap - EXIT INT TERM
)

corpus_is_current() {
  local case_dir=$1 command=$2
  [[ -f $case_dir/command ]] &&
    [[ $(<"$case_dir/command") == "$command" ]]
}

prune_removed_cases() {
  local case_dir name
  for case_dir in "$CORPUS_ROOT"/*; do
    [[ -d $case_dir ]] || continue
    name=${case_dir##*/}
    if ! jq -e --arg name "$name" 'has($name)' "$CATALOG" >/dev/null; then
      echo "removing $name"
      rm -rf -- "$case_dir"
    fi
  done
}

generate() {
  [[ -x $CORPUS_GEN ]] || {
    echo "error: $CORPUS_GEN not found; run 'make corpus-gen' first" >&2
    return 1
  }
  mkdir -p "$CORPUS_ROOT/.staging"

  local name command case_dir
  local -a failed=()
  while IFS=$'\t' read -r name command; do
    echo "==> $name"
    case_dir="$CORPUS_ROOT/$name"
    if [[ -d $case_dir ]]; then
      if corpus_is_current "$case_dir" "$command"; then
        echo "reusing $name"
        continue
      fi
      echo "removing outdated $name"
      rm -rf -- "$case_dir"
    fi
    if ! generate_case "$name" "$command"; then
      failed+=("$name")
    fi
  done < <(jq -r 'to_entries[] | [.key, .value] | @tsv' "$CATALOG")

  if ((${#failed[@]} > 0)); then
    echo "failed: ${failed[*]}" >&2
    return 1
  fi
  prune_removed_cases
  echo "corpora written to $CORPUS_ROOT"
}

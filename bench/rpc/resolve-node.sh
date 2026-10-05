# Sourced helper: resolve_node <name|url> sets NODE_URL, NODE_NAME and NODE_CFG.
# A name merges nodes/<name>.json with the gitignored nodes/<name>.local.json
# (either may be missing; local keys win) into NODE_CFG and reads its "url".
# A literal http(s):// URL is used as-is, slugified for NODE_NAME; NODE_CFG
# is empty.

NODES_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/nodes"

resolve_node() {
  local node=$1
  if [[ "$node" == http://* || "$node" == https://* ]]; then
    NODE_URL=$node
    NODE_NAME=$(printf '%s' "$node" | tr -cs 'a-zA-Z0-9._-' '-')
    NODE_CFG=
  else
    local cfgs=() cfg
    for cfg in "$NODES_DIR/$node.json" "$NODES_DIR/$node.local.json"; do
      if [[ -f $cfg ]]; then
        cfgs+=("$cfg")
      fi
    done
    if ((${#cfgs[@]} == 0)); then
      local known=("$NODES_DIR"/*.json)
      [[ -e ${known[0]} ]] || known=()
      known=("${known[@]##*/}")
      known=("${known[@]%.json}")
      known=("${known[@]%.local}")
      echo "error: unknown node '$node'; known: $(printf '%s\n' "${known[@]}" | sort -u | paste -sd' ')" >&2
      exit 1
    fi
    NODE_CFG=$(jq -n 'reduce inputs as $cfg ({}; . * $cfg)' "${cfgs[@]}") || exit 1
    NODE_URL=$(jq -r '.url // empty' <<<"$NODE_CFG")
    if [[ -z $NODE_URL ]]; then
      echo "error: node '$node' must set \"url\" in ${cfgs[*]}" >&2
      exit 1
    fi
    NODE_NAME=$node
  fi
}

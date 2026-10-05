# Sourced helper: resolve_node <name|url> sets NODE_URL and NODE_NAME.
# A name reads "url" from nodes/<name>.json; a literal http(s):// URL is
# used as-is, slugified for NODE_NAME.

NODES_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/nodes"

resolve_node() {
  local node=$1
  if [[ "$node" == http://* || "$node" == https://* ]]; then
    NODE_URL=$node
    NODE_NAME=$(printf '%s' "$node" | tr -cs 'a-zA-Z0-9._-' '-')
  else
    local cfg="$NODES_DIR/$node.json"
    if [[ ! -f $cfg ]]; then
      local known=("$NODES_DIR"/*.json)
      [[ -e ${known[0]} ]] || known=()
      known=("${known[@]##*/}")
      echo "error: unknown node '$node'; known: ${known[*]%.json}" >&2
      exit 1
    fi
    NODE_URL=$(jq -r '.url // empty' "$cfg")
    if [[ -z $NODE_URL ]]; then
      echo "error: $cfg must set \"url\"" >&2
      exit 1
    fi
    NODE_NAME=$node
  fi
}

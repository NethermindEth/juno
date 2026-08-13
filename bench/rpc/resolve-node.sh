# Sourced helper: resolve_node <name|url> sets NODE_URL and NODE_NAME.
# Names come from nodes.json (a flat map of node name -> JSON-RPC URL);
# a literal http(s):// URL is used as-is, slugified for NODE_NAME.

NODES_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/nodes.json"

resolve_node() {
  local node=$1
  if [[ "$node" == http://* || "$node" == https://* ]]; then
    NODE_URL=$node
    NODE_NAME=$(printf '%s' "$node" | tr -cs 'a-zA-Z0-9._-' '-')
  else
    NODE_URL=$(jq -r --arg n "$node" '.[$n] // empty' "$NODES_FILE")
    if [[ -z "$NODE_URL" ]]; then
      echo "error: unknown node '$node'; known: $(jq -r 'keys | join(", ")' "$NODES_FILE")" >&2
      exit 1
    fi
    NODE_NAME=$node
  fi
}

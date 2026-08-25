rpc_result() {
  local method=$1
  curl --fail --silent --show-error \
    --connect-timeout 5 --max-time "${RPC_TIMEOUT:-30}" \
    --header 'Content-Type: application/json' \
    --data "$(jq -cn --arg method "$method" \
      '{jsonrpc:"2.0",id:1,method:$method,params:[]}')" \
    "$NODE_URL" |
    jq -er 'if .error then error(.error.message // "JSON-RPC error") else .result end'
}

preflight() {
  rpc_result starknet_chainId >/dev/null
}

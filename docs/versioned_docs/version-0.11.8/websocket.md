---
title: WebSocket Interface
---

# WebSocket Interface :globe_with_meridians:

Juno provides a WebSocket RPC interface that supports all of [Starknet's JSON-RPC API](https://playground.open-rpc.org/?uiSchema%5BappBar%5D%5Bui:splitView%5D=false&schemaUrl=https://raw.githubusercontent.com/starkware-libs/starknet-specs/v0.7.0/api/starknet_api_openrpc.json&uiSchema%5BappBar%5D%5Bui:input%5D=false&uiSchema%5BappBar%5D%5Bui:darkMode%5D=true&uiSchema%5BappBar%5D%5Bui:examplesDropdown%5D=false) endpoints and allows you to [subscribe to newly created blocks](#subscribe-to-newly-created-blocks).

## Enable the WebSocket server

To enable the WebSocket RPC server, use the following configuration options:

- `ws`: Enables the Websocket RPC server on the default port (disabled by default).
- `ws-host`: The interface on which the Websocket RPC server will listen for requests. If skipped, it defaults to `localhost`.
- `ws-port`: The port on which the WebSocket server will listen for requests. If skipped, it defaults to `6061`.

```bash
# Docker container
docker run -d \
  --name juno \
  -p 6061:6061 \
  nethermind/juno \
  --ws \
  --ws-port 6061 \
  --ws-host 0.0.0.0

# Standalone binary
./build/juno --ws --ws-port 6061 --ws-host 0.0.0.0
```

## Making WebSocket requests

You can use any of [Starknet's Node API Endpoints](https://playground.open-rpc.org/?uiSchema%5BappBar%5D%5Bui:splitView%5D=false&schemaUrl=https://raw.githubusercontent.com/starkware-libs/starknet-specs/v0.7.0/api/starknet_api_openrpc.json&uiSchema%5BappBar%5D%5Bui:input%5D=false&uiSchema%5BappBar%5D%5Bui:darkMode%5D=true&uiSchema%5BappBar%5D%5Bui:examplesDropdown%5D=false) with Juno. Check the availability of Juno with the `juno_version` method:

```mdx-code-block
import Tabs from "@theme/Tabs";
import TabItem from "@theme/TabItem";
```

<Tabs>
<TabItem value="request" label="Request">

```json
{
  "jsonrpc": "2.0",
  "method": "juno_version",
  "params": [],
  "id": 1
}
```

</TabItem>
<TabItem value="response" label="Response">

```json
{
  "jsonrpc": "2.0",
  "result": "v0.11.7",
  "id": 1
}
```

</TabItem>
</Tabs>

Get the most recent accepted block hash and number with the `starknet_blockHashAndNumber` method:

<Tabs>
<TabItem value="request" label="Request">

```json
{
  "jsonrpc": "2.0",
  "method": "starknet_blockHashAndNumber",
  "params": [],
  "id": 1
}
```

</TabItem>
<TabItem value="response" label="Response">

```json
{
  "jsonrpc": "2.0",
  "result": {
    "block_hash": "0x637ae4d7468bb603c2f16ba7f9118d58c7d7c98a8210260372e83e7c9df443a",
    "block_number": 640827
  },
  "id": 1
}
```

</TabItem>
</Tabs>

## Subscribe to newly created blocks

The WebSocket server provides a `starknet_subscribeNewHeads` method that emits an event when new blocks are added to the blockchain:

<Tabs>
<TabItem value="request" label="Request">

```json
{
  "jsonrpc": "2.0",
  "method": "starknet_subscribeNewHeads",
  "params": [],
  "id": 1
}
```

</TabItem>
<TabItem value="response" label="Response">

```json
{
  "jsonrpc": "2.0",
  "result": 16570962336122680234,
  "id": 1
}
```

</TabItem>
</Tabs>

When a new block is added, you will receive a message like this:

```json
{
  "jsonrpc": "2.0",
  "method": "starknet_subscribeNewHeads",
  "params": {
    "result": {
      "block_hash": "0x840660a07a17ae6a55d39fb6d366698ecda11e02280ca3e9ca4b4f1bad741c",
      "parent_hash": "0x529ca67a127e4f40f3ae637fc54c7a56c853b2e085011c64364911af74c9a5c",
      "block_number": 65644,
      "new_root": "0x4e88ddf34b52091611b34d72849e230d329902888eb57c8e3c1b9cc180a426c",
      "timestamp": 1715451809,
      "sequencer_address": "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8",
      "l1_gas_price": {
        "price_in_fri": "0x3727bcc63f1",
        "price_in_wei": "0x5f438c77"
      },
      "l1_data_gas_price": {
        "price_in_fri": "0x230e40e8866c6e",
        "price_in_wei": "0x3c8c4a9ea51"
      },
      "l1_da_mode": "BLOB",
      "starknet_version": "0.13.1.1"
    },
    "subscription": 16570962336122680234
  }
}
```

## Unsubscribe from newly created blocks

Use the `juno_unsubscribe` method with the `result` value from the subscription response or the `subscription` field from any new block event to stop receiving updates for new blocks:

<Tabs>
<TabItem value="request" label="Request">

```json
{
  "jsonrpc": "2.0",
  "method": "juno_unsubscribe",
  "params": {
    "id": 16570962336122680234
  },
  "id": 1
}
```

</TabItem>
<TabItem value="response" label="Response">

```json
{
  "jsonrpc": "2.0",
  "result": true,
  "id": 1
}
```

</TabItem>
</Tabs>

## Testing the WebSocket connection

You can test your WebSocket connection using tools like [wscat](https://github.com/websockets/wscat) or [websocat](https://github.com/vi/websocat):

```bash
# wscat
$ wscat -c ws://localhost:6061
    > {"jsonrpc": "2.0", "method": "juno_version", "id": 1}
    < {"jsonrpc": "2.0", "result": "v0.11.7", "id": 1}

# websocat
$ websocat -v ws://localhost:6061
    [INFO  websocat::lints] Auto-inserting the line mode
    [INFO  websocat::stdio_threaded_peer] get_stdio_peer (threaded)
    [INFO  websocat::ws_client_peer] get_ws_client_peer
    [INFO  websocat::ws_client_peer] Connected to ws
    {"jsonrpc": "2.0", "method": "juno_version", "id": 1}
    {"jsonrpc": "2.0", "result": "v0.11.7", "id": 1}
```

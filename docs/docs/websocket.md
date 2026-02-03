---
title: WebSocket Interface
---

# WebSocket Interface :globe_with_meridians:

Juno provides a WebSocket RPC interface that supports all of [Starknet's JSON-RPC API](https://playground.open-rpc.org/?uiSchema%5BappBar%5D%5Bui:splitView%5D=false&schemaUrl=https://raw.githubusercontent.com/starkware-libs/starknet-specs/v0.8.1/api/starknet_api_openrpc.json&uiSchema%5BappBar%5D%5Bui:input%5D=false&uiSchema%5BappBar%5D%5Bui:darkMode%5D=true&uiSchema%5BappBar%5D%5Bui:examplesDropdown%5D=false) endpoints and allows you to [subscribe to newly created blocks](#subscribe-to-newly-created-blocks).

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
  --ws-host 0.0.0.0 \
  --eth-node <YOUR-ETH-NODE>

# Standalone binary
./build/juno --ws --ws-port 6061 --ws-host 0.0.0.0 --eth-node <YOUR-ETH-NODE>
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
  "result": "v0.14.3",
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
  "id": 1
}
```

</TabItem>
<TabItem value="response" label="Response">

```json
{
  "jsonrpc": "2.0",
  "result": 6178305545967232212,
  "id": 1
}
```

</TabItem>
</Tabs>

When a new block is added, you will receive a message like this:

```json
{
  "jsonrpc": "2.0",
  "method": "starknet_subscriptionNewHeads",
  "params": {
    "result": {
      "block_hash": "0x662757cbae602a3146cd96e5b661e92cf5d120ccc1d9ac6e78bee200afddfd5",
      "parent_hash": "0x348c37f50bf689c7fbf9ee971bf4378cf9232882e7a61eb2117486ee61236b1",
      "block_number": 69061,
      "new_root": "0x128eabdfcabc8043d1a7f30faed9fdc077e57fdce7aeb072c514b132e99c499",
      "timestamp": 1739977268,
      "sequencer_address": "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8",
      "l1_gas_price": {
        "price_in_fri": "0xd177c25056f4",
        "price_in_wei": "0x4769a28dd"
      },
      "l1_data_gas_price": {
        "price_in_fri": "0x963",
        "price_in_wei": "0x1"
      },
      "l1_da_mode": "BLOB",
      "starknet_version": "0.13.4",
      "l2_gas_price": {
        "price_in_fri": "0x157312ab4",
        "price_in_wei": "0x7500b"
      }
    },
    "subscription_id": 6178305545967232212
  }
}
```

## Subscribe to events

You can subscribe to events using the `starknet_subscribeEvents` method:

<Tabs>
<TabItem value="request" label="Request">

```json
{
  "jsonrpc": "2.0",
  "method": "starknet_subscribeEvents",
  "params": {
    "block_id": "latest"
  },
  "id": 1
}
```

</TabItem>
<TabItem value="response" label="Response">

```json
{
  "jsonrpc": "2.0",
  "result": 12301735893776740437,
  "id": 1
}
```

</TabItem>
</Tabs>

## Subscribe to transaction status

You can track the status of transactions using the `starknet_subscribeTransactionStatus` method:

<Tabs>
<TabItem value="request" label="Request">

```json
{
  "jsonrpc": "2.0",
  "method": "starknet_subscribeTransactionStatus",
  "params": {
    "transaction_hash": "0x22a6cd68819aa4813fed5db5cbaa0f396936b7bd53e4de51ef19ab57317de7c"
  },
  "id": 1
}
```

</TabItem>
</Tabs>

## Subscribe to pending transactions

You can subscribe to pending transactions using the `starknet_subscribePendingTransactions` method:

<Tabs>
<TabItem value="request" label="Request">

```json
{
  "jsonrpc": "2.0",
  "method": "starknet_subscribePendingTransactions",
  "id": 1
}
```

</TabItem>
</Tabs>

## Unsubscribe

Use the `starknet_unsubscribe` with subscription_id method to stop receiving updates for any of the subscribed events:

<Tabs>
<TabItem value="request" label="Request">

```json
{
  "jsonrpc": "2.0",
  "method": "starknet_unsubscribe",
  "params": {
    "subscription_id": 14754861534419680325
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
    < {"jsonrpc": "2.0", "result": "v0.14.3", "id": 1}

# websocat
$ websocat -v ws://localhost:6061
    [INFO  websocat::lints] Auto-inserting the line mode
    [INFO  websocat::stdio_threaded_peer] get_stdio_peer (threaded)
    [INFO  websocat::ws_client_peer] get_ws_client_peer
    [INFO  websocat::ws_client_peer] Connected to ws
    {"jsonrpc": "2.0", "method": "juno_version", "id": 1}
    {"jsonrpc": "2.0", "result": "v0.14.3", "id": 1}
```

## Supported Starknet API versions

Juno supports the following Starknet API versions:

- **v0.10.0**: Accessible via endpoint `/ws/v0_10`
- **v0.9.0**: Accessible via endpoint `/ws/v0_9`
- **v0.8.1**: Accessible via endpoint `/ws/v0_8`
- **v0.7.0**: Accessible via endpoint `/ws/v0_7`

To use a specific API version, specify the version endpoint in your WS calls:

<Tabs>
<TabItem value="v10" label="v0.10.0">

```bash
# wscat
$ wscat -c ws://localhost:6061/ws/v0_9
    > {"jsonrpc": "2.0", "method": "juno_version", "id": 1}
    < {"jsonrpc": "2.0", "result": "v0.15.18", "id": 1}
```

</TabItem>
<TabItem value="v9" label="v0.9.0">

```bash
# wscat
$ wscat -c ws://localhost:6061/ws/v0_9
    > {"jsonrpc": "2.0", "method": "juno_version", "id": 1}
    < {"jsonrpc": "2.0", "result": "v0.15.18", "id": 1}
```

</TabItem>
<TabItem value="v8" label="v0.8.0">

```bash
# wscat
$ wscat -c ws://localhost:6061/ws/v0_8
    > {"jsonrpc": "2.0", "method": "juno_version", "id": 1}
    < {"jsonrpc": "2.0", "result": "v0.15.18", "id": 1}
```

</TabItem>
<TabItem value="v7" label="v0.7.0">

```bash
# wscat
$ wscat -c ws://localhost:6061/ws/v0_7
    > {"jsonrpc": "2.0", "method": "juno_version", "id": 1}
    < {"jsonrpc": "2.0", "result": "v0.15.18", "id": 1}
```

</TabItem>
</Tabs>


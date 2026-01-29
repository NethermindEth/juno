---
title: JSON-RPC Interface
---

# JSON-RPC Interface :globe_with_meridians:

Interacting with Juno requires sending requests to specific JSON-RPC API methods. Juno supports all of [Starknet's Node API Endpoints](https://playground.open-rpc.org/?uiSchema%5BappBar%5D%5Bui:splitView%5D=false&schemaUrl=https://raw.githubusercontent.com/starkware-libs/starknet-specs/v0.8.1/api/starknet_api_openrpc.json&uiSchema%5BappBar%5D%5Bui:input%5D=false&uiSchema%5BappBar%5D%5Bui:darkMode%5D=true&uiSchema%5BappBar%5D%5Bui:examplesDropdown%5D=false) over HTTP and [WebSocket](websocket).

## Enable the JSON-RPC server

To enable the JSON-RPC interface, use the following configuration options:

- `http`: Enables the HTTP RPC server on the default port and interface (disabled by default).
- `http-host`: The interface on which the HTTP RPC server will listen for requests. If skipped, it defaults to `localhost`.
- `http-port`: The port on which the HTTP server will listen for requests. If skipped, it defaults to `6060`.

```bash
# Docker container
docker run -d \
  --name juno \
  -p 6060:6060 \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --eth-node <YOUR-ETH-NODE> \
  --http-host 0.0.0.0

# Standalone binary
./build/juno --http --http-port 6060 --http-host 0.0.0.0 --eth-node <YOUR-ETH-NODE>
```

## Making JSON-RPC requests

You can use any of [Starknet's Node API Endpoints](https://playground.open-rpc.org/?uiSchema%5BappBar%5D%5Bui:splitView%5D=false&schemaUrl=https://raw.githubusercontent.com/starkware-libs/starknet-specs/v0.8.1/api/starknet_api_openrpc.json&uiSchema%5BappBar%5D%5Bui:input%5D=false&uiSchema%5BappBar%5D%5Bui:darkMode%5D=true&uiSchema%5BappBar%5D%5Bui:examplesDropdown%5D=false) with Juno. Check the availability of Juno with the `juno_version` method:

```mdx-code-block
import Tabs from "@theme/Tabs";
import TabItem from "@theme/TabItem";
```

<Tabs>
<TabItem value="raw" label="Raw">

```json
{
  "jsonrpc": "2.0",
  "method": "juno_version",
  "params": [],
  "id": 1
}
```

</TabItem>
<TabItem value="curl" label="cURL">

```bash
curl --location 'http://localhost:6060/v0_9' \
--header 'Content-Type: application/json' \
--data '{
    "jsonrpc": "2.0",
    "method": "juno_version",
    "params": [],
    "id": 1
}'
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
<TabItem value="raw" label="Raw">

```json
{
  "jsonrpc": "2.0",
  "method": "starknet_blockHashAndNumber",
  "params": [],
  "id": 1
}
```

</TabItem>
<TabItem value="curl" label="cURL">

```bash
curl --location 'http://localhost:6060' \
--header 'Content-Type: application/json' \
--data '{
    "jsonrpc": "2.0",
    "method": "starknet_blockHashAndNumber",
    "params": [],
    "id": 1
}'
```

</TabItem>
<TabItem value="starknetjs" label="Starknet.js">

```js
const { RpcProvider } = require("starknet");

const provider = new RpcProvider({
  nodeUrl: "http://localhost:6060",
});

provider.getBlockLatestAccepted().then((blockHashAndNumber) => {
  console.log(blockHashAndNumber);
});
```

</TabItem>
<TabItem value="starknetgo" label="Starknet.go">

```go
package main

import (
	"context"
	"fmt"
	"log"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/starknet.go/rpc"
	"github.com/NethermindEth/starknet.go/utils"
)

func main() {
	rpcUrl := "http://localhost:6060"
	client, err := rpc.NewClient(rpcUrl)
	if err != nil {
		log.Fatal(err)
	}

	provider := rpc.NewProvider(client)
	result, err := provider.BlockHashAndNumber(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("BlockHashAndNumber:", result)
}
```

</TabItem>
<TabItem value="starknetrs" label="Starknet.rs">

```rust
use starknet::providers::{
    jsonrpc::{HttpTransport, JsonRpcClient},
    Provider, Url,
};

#[tokio::main]
async fn main() {
    let provider = JsonRpcClient::new(HttpTransport::new(
        Url::parse("http://localhost:6060").unwrap(),
    ));

    let result = provider.block_hash_and_number().await;
    match result {
        Ok(block_hash_and_number) => {
            println!("{block_hash_and_number:#?}");
        }
        Err(err) => {
            eprintln!("Error: {err}");
        }
    }
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

## Supported Starknet API versions

Juno supports the following Starknet API versions:

- **v0.10.0**: Accessible via endpoints `/v0_10`, `/rpc/v0_10`
- **v0.9.0**: Accessible via endpoints `/v0_9`, `/rpc/v0_9`
- **v0.8.1**: Accessible via endpoints `/v0_8` or `/rpc/v0_8`, or the default `/`
- **v0.7.0**: Accessible via endpoints `/v0_7`, `/rpc/v0_7`
- **v0.6.0**: Accessible via endpoints `/v0_6`, `/rpc/v0_6`

To use a specific API version, specify the version endpoint in your RPC calls:

<Tabs>

<TabItem value="v10" label="v0.10.0">

```bash
curl --location 'http://localhost:6060/v0_10' \
--header 'Content-Type: application/json' \
--data '{
    "jsonrpc": "2.0",
    "method": "starknet_chainId",
    "params": [],
    "id": 1
}'
```

</TabItem>


<TabItem value="v9" label="v0.9.0">

```bash
curl --location 'http://localhost:6060/v0_9' \
--header 'Content-Type: application/json' \
--data '{
    "jsonrpc": "2.0",
    "method": "starknet_chainId",
    "params": [],
    "id": 1
}'
```

</TabItem>

<TabItem value="v8" label="v0.8.1">

```bash
curl --location 'http://localhost:6060/v0_8' \
--header 'Content-Type: application/json' \
--data '{
    "jsonrpc": "2.0",
    "method": "starknet_chainId",
    "params": [],
    "id": 1
}'
```

</TabItem>

<TabItem value="v7" label="v0.7.0">

```bash
curl --location 'http://localhost:6060/v0_7' \
--header 'Content-Type: application/json' \
--data '{
    "jsonrpc": "2.0",
    "method": "starknet_chainId",
    "params": [],
    "id": 1
}'
```

</TabItem>

<TabItem value="v6" label="v0.6.0">

```bash
curl --location 'http://localhost:6060/v0_6' \
--header 'Content-Type: application/json' \
--data '{
    "jsonrpc": "2.0",
    "method": "starknet_chainId",
    "params": [],
    "id": 1
}'
```

</TabItem>

</Tabs>


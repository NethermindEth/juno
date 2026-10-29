---
title: Remote Database
description: "Run a Juno node without a local database. The node reads the database of another Juno node over gRPC."
---

# Remote Database

With the `--remote-db` option, a **reading node** reads the database of another, **serving** Juno node over gRPC instead.

## How it works

- The serving node syncs from Starknet and verifies blocks in the normal way. The `--grpc` option exposes its database to other nodes.
- The reading node connects to the serving node with the `--remote-db` option. It reads blocks, state, and classes from the serving node. It executes contract calls, fee estimations, and simulations on its own CPU.
- The connection is read only. A reading node cannot change the database of the serving node.
- The reading node polls the feeder gateway for the latest block header. It uses the header for `starknet_syncing` and for the `/ready` endpoint.

## Run the serving node

Start the serving node and enable the gRPC server:

```bash
./build/juno \
  --eth-node <YOUR-ETH-NODE> \
  --db-path <DB-PATH> \
  --grpc \
  --grpc-host <PRIVATE-IP> \
  --grpc-port 6064
```

The default `--grpc-host` is `localhost`, which accepts connections from the same machine only. Set it to an interface that the reading node can reach. See the [gRPC options](configuring#grpc) for the full list.

:::warning
The gRPC server has no authentication and no encryption, like the JSON-RPC server.
:::

## Run the reading node

Start the reading node with the address of the serving node:

```bash
./build/juno \
  --remote-db <SERVING-HOST>:6064 \
  --disable-l1-verification \
  --http \
  --http-host 0.0.0.0 \
  --http-port 6060
```

Apply these rules on the reading node:

- Use the same `--network` as the serving node.
- Use the same Juno version as the serving node. The reading node skips database migrations and reads the database in the format of the serving node.
- Set `--disable-l1-verification`. The serving node verifies L1 and stores the result in the database, where the reading node reads it.

## Limitations

- Every database read is a network round trip. Reads on the reading node are much slower than on the serving node.
- The serving node does the database work for every read, so a reading node adds load to the serving node.
- The reading node does not poll the pre-confirmed block. Requests for the `pre_confirmed` block return an empty block whose parent is the latest block.
- The reading node does not store blocks, so it has no new-block events. WebSocket subscriptions on it send the initial data, such as the blocks or events from the given `block_id` to the latest block, but no live updates.

## When the serving node is unavailable

The serving node must be reachable when the reading node starts. If it is not, the reading node stops with an error. After the start, the reading node continues to run and connects again when the serving node is available. While the serving node is unavailable:

- RPC requests return errors.
- `/ready` and `/ready/rpc` return `503`, so a load balancer sends no more traffic to the reading node.
- `/live` returns `200`, so Kubernetes does not restart the container.

See [Health endpoints](monitoring#health-endpoints-for-kubernetes) for probe examples.

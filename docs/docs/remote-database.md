---
title: Remote Database
description: "Run a Juno node with no database of its own by reading another node's over gRPC."
---

A Juno node normally keeps its own database on local disk, built by
[syncing](running-juno) or restored from a [snapshot](snapshots). With
`--remote-db` it reads from another Juno node over gRPC instead, so one node
owns the data and others run without a copy of it.

:::info
Both nodes need Juno v0.16.7-rc.0 or later. Support spans the gRPC server on the
serving node and the database client on the reading node, so upgrade both.

No stable release and no published Docker image carry it yet, so
[building from source](running-juno#building-from-source) is the only way to run
this today. Check out the tag first, or you will build whatever your working copy
points at:

```bash
git checkout v0.16.7-rc.0
make juno
```
:::

![Node A syncs from Starknet and serves its database over gRPC on port 6064. Node B reads from Node A and serves JSON-RPC.](/img/remote-database.svg)

## When to use it

The reading node has no database to build and nothing to sync, so it serves
requests within seconds of starting. That suits short-lived or scaled-to-zero
instances, and moves RPC handling and contract execution off the node that owns
the data.

:::caution
Every read the reading node answers is fetched from the serving node's database,
so the serving node does that work either way. If it is already saturating on
reads, add a [second full node](hardware-requirements) instead.
:::

What to plan for:

- **Keep the two nodes close.** Every read is a round trip, so same datacentre
  or availability zone.
- **Allow the reading node outbound HTTPS to the feeder gateway.** It polls once
  a minute for the latest block header. That is
  `feeder.alpha-mainnet.starknet.io` on mainnet,
  `feeder.alpha-sepolia.starknet.io` on Sepolia.

## Serving a database

On the node that owns the data:

```bash
juno --network <NETWORK> \
  --eth-node <YOUR-ETH-NODE> \
  --grpc --grpc-host <PRIVATE-IP> --grpc-port 6064
```

| Flag | Default | Description |
| - | - | - |
| `grpc` | `false` | Enable the gRPC server |
| `grpc-host` | `localhost` | Interface the gRPC server listens on |
| `grpc-port` | `6064` | Port the gRPC server listens on |

The default host accepts connections from the same machine only, so set
`--grpc-host` to an interface the reading node can reach. Prefer a private
interface over `0.0.0.0`, and restrict the port to the reading nodes. See all flags in
[configuration options](configuring#configuration-options).

:::warning
The gRPC connection is neither encrypted nor authenticated, and Juno has no
option to change that. Anyone who can reach the port can read your entire
database, and anyone on the network path can read the traffic. Keep it on a
private network, never on the internet.
:::

## Reading from a remote database

On the reading node, point `--remote-db` at the serving node. No volume is
needed, because nothing is written to disk:

```bash
juno --network <NETWORK> \
  --remote-db <SERVING-HOST>:6064 \
  --http --http-host 0.0.0.0 --http-port 6060
```

:::caution
Point a readiness probe at `/ready/rpc`, not `/ready` or `/ready/sync`.

Those two report on sync progress, so they return 503 whenever the served data
is behind the chain head. That includes a serving node that is still catching up,
and a reading node that cannot reach the feeder gateway and so never learns what
the head is. Either way reads keep working while both endpoints stay at 503.
`/ready/rpc` reports on the database instead. See
[health endpoints](monitoring#health-endpoints-for-kubernetes).
:::

Pass `--remote-db` a plain `host:port` with no scheme. A DNS name works as well
as an address. Use the same `--network` as the serving node, and set
`--http-host` for the same reason you set `--grpc-host` above. `--remote-db`
replaces the local database, so `--db-path` is unused. The connection is read
only, so a reading node cannot alter the data it is served.

A reading node answers the same [JSON-RPC methods](json-rpc) as any other node,
including `starknet_call` and the others that execute contracts. It runs them
itself and reads the state they need from the serving node.

It still runs its synchronizer, but only to follow the chain head, because the
serving node already fetched and verified the blocks. Since it needs that much,
`--remote-db` cannot be combined with `--disable-sync`. Juno rejects the pair at
startup.

## When the serving node is unavailable

The reading node stays up and recovers on its own, so there is nothing to
restart. Reads resume as soon as the serving node is back.

While the serving node is down, or before it has synced its first block:

- RPC requests return a JSON-RPC error rather than an empty result that looks
  like an answer. `There are no blocks` and `Internal error` have both been
  observed, and the error you get depends on the method.
- `/ready/rpc` returns 503, which takes the node out of rotation behind a load
  balancer.
- `/live` stays 200, so an orchestrator leaves the container running.
- The reading node logs nothing during the outage or the reconnect, so the
  readiness endpoint is your only signal.

## What this is not

The gRPC server exposes one service with two methods, `Version` and `Tx`. It
exists so one Juno node can serve its database to another, and Juno is its only
consumer. It is not a general purpose indexing or state query interface, and not
meant for third party clients.

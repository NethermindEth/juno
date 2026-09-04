---
slug: /
title: Introduction
---

# Welcome to Juno

Juno is a Go implementation of a Starknet full-node client created by Nethermind to allow node operators to easily and reliably support the network and advance its decentralisation goals. Juno supports various node setups, from casual to production-grade indexers.

- **[Small database](configuring#pruning)**: pruning runs mainnet in roughly a fifth of full-history disk
- **Ultra-fast synchronisation**: Limited only by your hardware and the sequencer.
- **[Snapshot sync](snapshots)**: start from a recent block instead of replaying from genesis
- **[Three RPC versions](json-rpc)**: `v0_8`, `v0_9` and `v0_10` served at the same time
- **[WebSocket subscriptions](websocket)**: new blocks and events pushed as they happen
- **[Minimal RPC latency](configuring#vm--compilation)**: tune VM concurrency and cache size for read-heavy nodes
- **[Prometheus metrics](monitoring)**: sync, database and RPC counters on a scrape endpoint

## Getting started

These pages follow the order you will need them, from sizing a machine to serving your first request.

```mdx-code-block
import GuideCard from '@site/src/components/GuideCard';

<GuideCard
  href="hardware-requirements"
  title="Hardware Requirements"
  description="Size a machine for a validator, a dApp backend, or an RPC provider"
/>

<GuideCard
  href="running-juno"
  title="Running a Juno Node"
  description="Start a node with Docker, a prebuilt binary, or from source"
/>

<GuideCard
  href="snapshots"
  title="Syncing from a Snapshot"
  description="Start from a recent block instead of replaying the chain from genesis"
/>

<GuideCard
  href="configuring"
  title="Configuring Juno"
  description="Set options with flags, environment variables, or a YAML file"
/>

<GuideCard
  href="json-rpc"
  title="Interacting with Juno"
  description="Call the node over JSON-RPC and subscribe to updates over WebSocket"
/>
```

## Community and support

The [FAQ](faq) covers the errors most operators meet first. For anything else:

- [Telegram](https://t.me/+LHRF4H8iQ3c5MDY0): Ask questions and follow announcements.
- [Discord](https://discord.gg/SZkKcmmChJ): Reach the Juno team and other operators.
- [X (Twitter)](https://x.com/Nethermind): Follow for the latest news and insights from Nethermind.

## Contributions and partnerships

We value community contributions and are eager to support your involvement. Here's how you can contribute:

- [Run a Juno node](running-juno) to strengthen the Starknet network, or [stake and validate](staking-validator).
- Give Juno a [star on GitHub](https://github.com/NethermindEth/juno).
- Share your thoughts on [X (Twitter)](https://twitter.com/intent/tweet?url=https%3A%2F%2Fgithub.com%2FNethermindEth%2Fjuno&via=Nethermind&hashtags=Starknet%2CJuno).
- [Report bugs](https://github.com/NethermindEth/juno/issues/new) or [suggest new features](https://github.com/NethermindEth/juno/issues/new).

:::tip
If you want to contribute but are unsure where to start, ask in [Discord](https://discord.gg/SZkKcmmChJ) and we'll point you at a beginner-friendly issue.
:::

Whether it's a partnership, an idea, or a question that doesn't fit anywhere above, we'd love to hear from you at [juno@nethermind.io](mailto:juno@nethermind.io).

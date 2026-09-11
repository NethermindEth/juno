---
slug: /
title: Introduction
description: "What Juno is and where to start: hardware, running a node, snapshots, configuration, and the JSON-RPC interface."
---

# Welcome to Juno

Juno is a Go implementation of a Starknet full-node client created by Nethermind to allow node operators to easily and reliably support the network and advance its decentralisation goals. Juno supports various node setups, from casual to production-grade indexers.

- **Small database footprint**: Committed to keeping the database size as small as possible
- **Ultra-fast synchronisation**: Limited only by your hardware and the sequencer.
- **Complete [JSON-RPC spec](https://github.com/starkware-libs/starknet-specs/tree/master) compliance**: Everything Starknet, accessible from a single point.
- **Minimal RPC response latency**: Ensuring your applications run smoothly.
- **Websocket interface**: For seamless real-time updates of the network.

## Getting started

Learn how to configure and manage your Juno node with the following resources:

```mdx-code-block
import GuideCard from '@site/src/components/GuideCard';

<GuideCard
  href="running-juno"
  icon="🚀"
  title="Running a Juno Node"
  description="Learn how to set up and operate your own Juno node"
/>

<GuideCard
  href="configuring"
  icon="⚙️"
  title="Juno Configuration Options"
  description="Explore various configuration options to customise your node"
/>

<GuideCard
  href="json-rpc"
  icon="🌐"
  title="Interacting with Juno"
  description="Discover how to interact with Juno using the JSON-RPC and WebSocket interfaces"
/>

<GuideCard
  href="snapshots"
  icon="📸"
  title="Syncing quickly from a Snapshot"
  description="Download and use a snapshot to quickly sync your node with the network"
/>
```

## Community and support

Join our community for support, engaging discussions, and updates:

- [Telegram](https://t.me/+LHRF4H8iQ3c5MDY0): Share ideas and stay informed with fellow Juno users.
- [Discord](https://discord.gg/SZkKcmmChJ): Connect in real-time with the Juno team and community.
- [X (Twitter)](https://x.com/NethermindStark): Follow for the latest news and insights from Nethermind.

## Contributions and partnerships

We value community contributions and are eager to support your involvement. Here’s how you can contribute:

- [Run a Juno node](running-juno) to strengthen the Starknet network.
- Give Juno a [star on GitHub](https://github.com/NethermindEth/juno/stargazers).
- Share your thoughts on [X (Twitter)](https://twitter.com/intent/tweet?url=https%3A%2F%2Fgithub.com%2FNethermindEth%2Fjuno&via=nethermindeth&text=Juno%20is%20Awesome%2C%20they%20are%20working%20hard%20to%20bring%20decentralization%20to%20StarkNet&hashtags=StarkNet%2CJuno%2CEthereum).
- [Report bugs](https://github.com/NethermindEth/juno/issues/new) or [suggest new features](https://github.com/NethermindEth/juno/issues/new).
- Encourage others to explore and use Juno.

:::tip
If you're ready to make PRs but unsure where to start, join our [Discord](https://discord.gg/TcHbSZ9ATd), and we'll guide you through some beginner-friendly issues.
:::

If you're interested in forming a partnership with the Juno team or have any suggestions or special requests, please don't hesitate to contact us via juno@nethermind.io

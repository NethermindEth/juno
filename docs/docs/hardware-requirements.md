---
title: Hardware Requirements
description: "CPU, memory and storage needed to run Juno, for validators and dApps or as a high-traffic RPC provider."
---

# Hardware Requirements

Juno can be used either as part of a **validator** setup during Starknet staking v2 ([read more](https://nethermindeth.github.io/starknet-staking-v2/)) or as a **full node** serving RPC requests. Hardware requirements will vary depending on the intended usage.

Each hardware component impacts different aspects of node performance:

- **High-speed CPU cores** allow the node to execute Cairo-heavy RPC methods more quickly such as `starknet_traceTransaction` or `starknet_estimateFee`.
- **Multiple CPU cores** (or threads) enable Juno to perform more tasks concurrently, which becomes especially important when serving a high volume of CPU-heavy requests (e.g. simulating execution / compiling Cairo classes).
- **More RAM** allows handling of many concurrent requests.
- **Fast SSD storage** improves the overall node performance. Nearly all internal processes require reading data (for RPC purposes) and writing data (during syncing).

:::tip
Remember to always pair your hardware accordingly. Having a very powerful CPU will provide minimal improvements if paired with a disk with slow read and write speeds.
:::

## Normal Usage (Validators, dApps)

These requirements are enough to comfortably run a Juno node. They will allow the node to keep in sync as well as performing validation duties. Additionally, it will be well capable of serving RPC request needs for individuals or small groups.

- **CPU**: 4 CPU cores
- **RAM**: 8GB or more
- **Storage**: High-speed NVMe SSD drive

:::tip
Additionally, if your app only requires access to _recent data_ you can set the `--prune-mode` flag to keep only recent history, reducing space requirements.
:::

## Heavy Usage (RPC Providers, Starknet Explorers)

With this configuration it will be possible for Juno nodes to work as servers to satisfy multiple RPC requests.

- **CPU**: 8 high-speed CPU cores
- **RAM**: 16GB of RAM
- **Storage**: Highest speed NVMe SSD drive

These requirements are a good baseline and should be adjusted to the node's dominant workload:
- **Increase CPU** if re-executing many transactions / blocks or compiling new Cairo classes.
- **Increase memory** if you want to hold a bigger chunk of the database in memory and minimize latency when fetching historical data.

See [Tuning](tuning) for more information on how to control the resource usage of your Juno node.

:::tip
We intend the above specifications as a guideline. You should set the hardware requirements that fit best for your usage. If unsure, feel free to [reach the team](https://juno.nethermind.io/#community-and-support)!
:::

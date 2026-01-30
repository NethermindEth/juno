---
title: Hardware Requirements
---

# Hardware Requirements :computer:

Juno can be used either as part of a **validator** setup during Starkent staking v2 ([read more](https://nethermindeth.github.io/starknet-staking-v2/)) or as a **full node** serving RPC requests. Hardware requirements will vary depending on the intended usage.

Each hardware component impacts different aspects of node performance:
- **High-speed CPU cores** allow the node to execute Cairo-heavy RPC methods more quickly such as `starknet_traceTransaction` or `starknet_estimateFee`. 
- **Multiple CPU cores** (or threads) enable Juno to perform more tasks concurrently, which becomes especially important when serving a high volume of RPC requests.
- **More RAM** reduces the likelihood of slowdowns when handling multiple data-intensive RPC requests.
- **Fast SSD storage** significantly improves the overall node performance. Nearly all internal processes require reading data (for RPC purposes) and writing data (during syncing). Faster disk I/O directly translates into faster request handling and synchronization. 

:::tip
Remember to always pair your hardware accordingly. Having a very powerful CPU will provide minimal improvements if paired with a disk with slow read and write speeds. 
:::

## Minimum requirements (Validators)

These requirements are the absolute minimum to comfortably run a Juno node. They will allow the node to keep in sync as well as performing validation duties. Additionally, it will be well capable of serving RPC request needs for individuals or small groups. 

- **CPU**: 4 CPU cores
- **RAM**: 8GB or more
- **Storage**: High-speed NVMe SSD drive

## Recommended requirements (RPC providers)

With this configuration it will be possible for Juno nodes to work as servers to satisfy multiple RPC requests.

- **CPU**: 16 high-speed CPU cores  
- **RAM**: 64GB of RAM
- **Storage**: Highest speed NVMe SSD drive

:::tip
We intend the above specifications as a guideline. You should set the hardware requirements that fit best for your usage. If unsure, feel free to [reach the team](https://juno.nethermind.io/#community-and-support)!
:::

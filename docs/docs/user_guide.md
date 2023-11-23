---
slug: /
sidebar_position: 1
title: Introduction
---


*Juno is your fast and featureful Starknet client implementation.*

Suitable for casual setups, production-grade indexers, and everything in between.

## The Role of Full Nodes in Blockchains
To appreciate Juno's significance, it is imperative to comprehend the pivotal role of full nodes within the blockchain ecosystem. Full nodes serve two fundamental functions:

- **Blockchain Synchronization:** Full nodes are entrusted with the task of downloading and archiving blocks, transactions, and state data, thereby maintaining an unalterable ledger of historical transactions.

- **RPC Service:** These nodes facilitate the dissemination of blockchain data through RPC endpoints, enabling various applications, wallets, and end-users to access this data. Additionally, they serve as conduits for transaction submission and network-wide broadcasting.

## Juno's Pioneering Contribution to StarkNet
Juno aspires to make substantial contributions to the decentralized architecture of StarkNet. At present, Juno boasts the following features:

- :racing_car: **JSON RPC Compatibility:** Juno diligently adheres to the JSON RPC specification, extending support for read, write, and trace APIs and providing minimal RPC respomse latency. With a **100% [JSON-RPC spec](https://github.com/starkware-libs/starknet-specs/tree/master) compliance**

- :zap: **Reorg Resolution on L2:** This client is adept at resolving reorganizations on Layer 2.

- :floppy_disk: **Database Migrations:** Juno seamlessly facilitates database migrations, ensuring a seamless transition when transitioning to different versions.

- :mag_right: **Database Query via gRPC:** Juno provides :mag_right Low-level GRPC database API for the most demanding workloads giving noteworthy performance advantages. This feature is particularly beneficial for intricate indexing tasks.


## Synchronization and Performance

Juno can swiftly synchronize approximately blocks, at a blazingly fast speed. It is crucial to note that performance metrics may fluctuate based on hardware configurations.

## Juno's Insights on Shaping the Blockchain Landscape

### Transaction Handling
- Juno assumes the responsibility of forwarding submitted transactions to the centralized gateway. This stems from the current StarkNet ecosystem's reliance on the gateway for block construction. As decentralization advances, Juno will introduce a local mempool for transaction management.

### Block Synchronization
- Juno's core focus lies in the efficient synchronization of blocks from the sequencer providing blazing fast sync. Four key optimizations that empower Juno to expedite this process:

#### Optimization 1: Block and State Diff Downloads
- Juno initiates its synchronization by downloading the block and state diff from StarkNet. Given StarkNet's Validity Rollup framework, Juno abstains from local transaction execution, placing trust in the sequencer's execution results.

#### Optimization 2: Pipelined Synchronization
- Juno streamlines the synchronization process through a pipelined approach. While rigorously verifying one block and its accompanying state diff, it concurrently conducts sanity checks on the subsequent block and initiates data retrieval for the following blocks.

#### Optimization 3: Efficient Database Management
- Juno excels at efficient storage management through a unique approach to contract storage. The introduction of parallelized root calculation for storage has significantly bolstered block synchronization speed.

#### Optimization 4: Mutable Trie Implementation
- Juno leverages a mutable trie approach, a concept pioneered by Aragon in the Ethereum ecosystem. This strategy optimizes storage by reducing data redundancy, a critical aspect of minimizing database size.

## Querying Juno
To access data from Juno, users can submit RPC requests that interact with the local database. Additionally, Juno extends support for gRPC, offering a more efficient querying mechanism that leverages Juno's database layout directly.
Example:

```shell
curl --location 'http://localhost:6060' \
--data '{
    "jsonrpc":"2.0",
     "method":"starknet_syncing",
    "id":1
}'
```

## Future Developments
Prospects for Juno are promising, with a steadfast commitment to advancing StarkNet's decentralization and introducing noteworthy features, including P2P (peer-to-peer) and SnapSync.

### Decentralization Through P2P
- Juno is actively engaged in the implementation of P2P and SnapSync.

### Ongoing Optimizations
- Juno is in relentless pursuit of further enhancements, including parallelization of state trie updates and other optimizations to elevate its functionality.


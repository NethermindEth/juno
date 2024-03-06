<p align="center">
  <a href="https://github.com/NethermindEth/juno">
    <img alt="Juno Logo" height="125" src="./.github/Juno_Light.png">
  </a>
  <br>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/NethermindEth/juno">
    <img src="https://pkg.go.dev/badge/github.com/NethermindEth/juno.svg">
  </a>
  <a href="https://goreportcard.com/report/github.com/NethermindEth/juno">
    <img src="https://goreportcard.com/badge/github.com/NethermindEth/juno">
  </a>
  <a href="https://github.com/NethermindEth/juno/actions">
    <img src="https://github.com/NethermindEth/juno/actions/workflows/juno-test.yml/badge.svg">
  </a>
  <a href="https://codecov.io/gh/NethermindEth/juno">
    <img src="https://codecov.io/gh/NethermindEth/juno/branch/main/graph/badge.svg">
  </a>

</p>
<p align="center">
  <a href="https://discord.gg/TcHbSZ9ATd">
    <img src="https://img.shields.io/badge/Discord-5865F2?style=for-the-badge&logo=discord&logoColor=white">
  </a>
  <a href="https://twitter.com/NethermindStark">
    <img src="https://img.shields.io/badge/Twitter-1DA1F2?style=for-the-badge&logo=twitter&logoColor=white">
  </a>
  <a href="https://t.me/+skAz9cUvo_AzZWM8">
    <img src="https://img.shields.io/badge/Telegram-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white">
  </a>
</p>


<p align="center">
  <b>Juno</b> is a golang <a href="https://starknet.io/">Starknet</a> node implementation by <a href="https://nethermind.io/">Nethermind</a> with the aim of decentralising Starknet.
</p>

> **📌 Looking for a Starknet RPC Provider?**
>
> Access Nethermind's Starknet RPC service for free at https://data.voyager.online.

## ⚙️ Installation

### Prerequisites

- Golang 1.20 or higher is required to build and run the project. You can find the installer on
  the official Golang [download](https://go.dev/doc/install) page.
- [Rust](https://www.rust-lang.org/tools/install).
- A C compiler: `gcc` or `clang`.

### Build and Run

```shell
make juno
./build/juno
```
Use the `--help` flag for configuration information.
Flags and their values can also be placed in a `.yaml` file that is passed in through `--config`.

### Run with Docker

To run Juno with Docker, use the following command. Make sure to create the `$HOME/juno` directory on your local machine before running the command.

```shell
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/juno:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --eth-node <YOUR-ETH-NODE>
```

You should replace `<YOUR-ETH-NODE> `with your actual Ethereum node address.
If you're using Infura, your Ethereum node address might look something like: `wss://mainnet.infura.io/ws/v3/your-infura-project-id`.
Make sure you are using the websocket URL `ws`/`wss` and not the http URL `http`/`https`.

To view logs from the Docker container, use the following command:

```shell
docker logs -f juno
```

## 📸 Snapshots

Use the provided snapshots to quickly sync your Juno node with the current state of the network. 

#### Mainnet

| Version | Size | Block | Download Link |
| ------- | ---- | ----- | ------------- |
| **>=v0.9.2**  | **156 GB** | **519634** | [**juno_mainnet.tar**](https://juno-snapshots.nethermind.dev/mainnet/juno_mainnet_v0.9.3_519634.tar) |

#### Goerli

| Version | Size | Block | Download Link |
| ------- | ---- | ----- | ------------- |
| **>=v0.9.2** | **48 GB** | **931113** | [**juno_goerli.tar**](https://juno-snapshots.nethermind.dev/goerli/juno_goerli_v0.9.2_931113.tar) |

### Run Juno Using Snapshot

1. **Download Snapshot**

   Fetch the snapshot from the provided URL:

   ```bash
   wget -O juno_mainnet.tar https://juno-snapshots.nethermind.dev/mainnet/juno_mainnet_v0.9.3_519634.tar
   ```

2. **Prepare Directory**

   Ensure you have a directory at `$HOME/snapshots`. If it doesn't exist yet, create it:

   ```bash
   mkdir -p $HOME/snapshots
   ```

3. **Extract Tarball**

   Extract the contents of the `.tar` file:

   ```bash
   tar -xvf juno_mainnet.tar -C $HOME/snapshots
   ```

4. **Run Juno**

   Execute the Docker command to run Juno, ensuring that you're using the correct snapshot path `$HOME/snapshots/juno_mainnet`:

   ```bash
   docker run -d \
     --name juno \
     -p 6060:6060 \
     -v $HOME/snapshots/juno_mainnet:/var/lib/juno \
     nethermind/juno \
     --http \
     --http-port 6060 \
     --http-host 0.0.0.0 \
     --db-path /var/lib/juno \
     --eth-node <YOUR-ETH-NODE>
   ```

After following these steps, Juno should be up and running on your machine, utilizing the provided snapshot.

## ✔ Supported Features

- Starknet [v0.13.1](https://docs.starknet.io/documentation/starknet_versions/version_notes/) support.
- JSON-RPC [v0.7.0](https://github.com/starkware-libs/starknet-specs/releases/tag/v0.7.0-rc2) (Available under `/v0_7` and default`/` endpoints)
  - `starknet_chainId`
  - `starknet_blockNumber`
  - `starknet_blockHashAndNumber`
  - `starknet_getBlockWithTxHashes`
  - `starknet_getBlockWithTxs`
  - `starknet_getBlockWithReceipts`
  - `starknet_getTransactionByHash`
  - `starknet_getTransactionReceipt`
  - `starknet_getBlockTransactionCount`
  - `starknet_getTransactionByBlockIdAndIndex`
  - `starknet_getStateUpdate`
  - `starknet_getNonce`
  - `starknet_getStorageAt`
  - `starknet_getTransactionStatus`
  - `starknet_getClassHashAt`
  - `starknet_getClass`
  - `starknet_getClassAt`
  - `starknet_syncing`
  - `starknet_getEvents`
  - `starknet_call`
  - `starknet_estimateFee`
  - `starknet_addInvokeTransaction`
  - `starknet_addDeclareTransaction`
  - `starknet_addDeployAccountTransaction`
  - `starkent_estimateMessageFee`
  - `starknet_traceTransaction`
  - `starknet_traceBlockTransactions`
  - `starknet_simulateTransactions`
  - `starknet_specVersion` 
  
- Juno's JSON-RPC:
  - `juno_version`
- JSON-RPC [v0.6.0](https://github.com/starkware-libs/starknet-specs/releases/tag/v0.6.0) (Available under `/v0_6` endpoint)
- Integration of CairoVM. 
- Verification of State from L1.
- Handle L1 and L2 Reorgs.
- Starknet state construction and storage using a path-based Merkle Patricia trie.
- Feeder gateway synchronisation of Blocks, Transactions, Receipts, State Updates and Classes.
- Block and Transaction hash verification.

## 🛣 Roadmap

### Phase 1: Permissionless access to Starknet ✅

<details>
<summary></summary>

* [X] Flat DB implementation of trie
* [X] Go implementation of crypto primitives
  * [X] Pedersen hash
  * [X] Starknet_Keccak
  * [X] Felt
* [X] Feeder gateway synchronisation
  * [X] State Update
  * [X] Blocks
  * [X] Transactions
  * [X] Class
* [X] Implement the following core data structures, and their Hash calculations
  * [X] Blocks
  * [X] Transactions and Transaction Receipts
  * [X] Contracts and Classes
* [X] Storing blocks, transactions and State updates in a local DB
* [X] Basic RPC (in progress)
  * [X] `starknet_chainId`
  * [X] `starknet_blockNumber`
  * [X] `starknet_blockHashAndNumber`
  * [X] `starknet_getBlockWithTxHashes`
  * [X] `starknet_getBlockWithTxs`
  * [X] `starknet_getTransactionByHash`
  * [X] `starknet_getTransactionReceipt`
  * [X] `starknet_getBlockTransactionCount`
  * [X] `starknet_getTransactionByBlockIdAndIndex`
  * [X] `starknet_getStateUpdate`

</details>

### Phase 2: Full JSON RPC Support ✅

<details>
<summary></summary>

The focus of Phase 2 will be to Verify the state from layer 1 and implement the remaining JSON-RPC endpoints.

* [X] Starknet v0.11.0 support
    * [X] Poseidon state trie support
* [X] Blockchain: implement blockchain reorganization logic.
* [X] Synchronisation: implement verification of state from layer 1.
* [X] JSON-RPC API [v0.3.0](https://github.com/starkware-libs/starknet-specs/releases/tag/v0.3.0):
    * [X] Implement the remaining endpoints:
        * [X] `starknet_syncing`
        * [X] `starknet_getNonce`
        * [X] `starknet_getStorageAt`
        * [X] `starknet_getClassHashAt`
        * [X] `starknet_getClass`
        * [X] `starknet_getClassAt`
        * [X] `starknet_getEvents`
* [X] Integration of CairoVM:
  * [X] `starknet_call`
  * [X] `starknet_estimateFee`
* [X] JSON-RPC Write API [v0.3.0](https://github.com/starkware-libs/starknet-specs/releases/tag/v0.3.0):
    * [X] `starknet_addInvokeTransaction`
    * [X] `starknet_addDeclareTransaction`
    * [X] `starknet_addDeployAccountTransaction`

</details>

### Phase 3: Starknet decentralization begins 🚧

<details>
<summary></summary>

Juno can synchronize Starknet state from other full nodes with the aim of decentralizing Starknet by removing the dependency from the centralized sequencer.


Snap sync is implemented, significantly reducing sync times.

</details>
  
### Phase 4: Juno becomes a Starknet Sequencer 🔜

<details>
<summary></summary>

The decentralization of Starknet is complete! Juno becomes a sequencer and participates in L2 consensus to secure the network. Juno has multiple modes of operation:
‍

•   Light client: provides fast permissionless access to Starknet with minimal verification.

•   Full Node: complete verification of Starknet state along with transaction execution.

•   Sequencer: secure the network by taking part in the L2 consensus mechanism.

</details>


## 👍 Contribute

We welcome PRs from external contributors and would love to help you get up to speed.
Let us know you're interested in the [Discord server](https://discord.gg/TcHbSZ9ATd) and we can discuss good first issues.
There are also many other ways to contribute. Here are some ideas:

* Run a node.
* Add a [GitHub Star](https://github.com/NethermindEth/juno/stargazers) to the project.
* [Tweet](https://twitter.com/intent/tweet?url=https%3A%2F%2Fgithub.com%2FNethermindEth%2Fjuno&via=nethermindeth&text=Juno%20is%20Awesome%2C%20they%20are%20working%20hard%20to%20bring%20decentralization%20to%20StarkNet&hashtags=StarkNet%2CJuno%2CEthereum) about Juno.
* Add a Github issue if you find a [bug](https://github.com/NethermindEth/juno/issues/new?assignees=&labels=&template=bug_report.md&title=), or you need or want a new [feature](https://github.com/NethermindEth/juno/issues/new?assignees=&labels=&template=feature_request.md&title=).

## 🤝 Partnerships

To establish a partnership with the Juno team, or if you have any suggestion or special request, feel free to reach us
via [email](mailto:juno@nethermind.io).

## ⚠️ License

Copyright (c) 2022-present, with the following [contributors](https://github.com/NethermindEth/juno/graphs/contributors).

Juno is open-source software licensed under the [Apache-2.0 License](https://github.com/NethermindEth/juno/blob/main/LICENSE).

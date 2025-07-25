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
  <a href="https://github.com/NethermindEth/juno/actions/workflows/juno-test.yml">
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


> ℹ️ **Note**: 
> If you're looking to become a <b>Starknet Validator</b>, you’ll also need to run a <a href="https://starknet.io/">validation tool</a>. We’ve prepared a <a href="https://nethermindeth.github.io/starknet-staking-v2/">guide</a> with detailed instructions. If you run into any issues, feel free to <a href="#📞-contact-us">reach out to us</a>.

## ⚙️ Installation

### Prerequisites

- Golang 1.24 or higher is required to build and run the project. You can find the installer on
  the official Golang [download](https://go.dev/doc/install) page.
- [Rust](https://www.rust-lang.org/tools/install) 1.86.0 or higher.
- A C compiler: `gcc`.
- Install some dependencies on your system:
  
  - macOS

    ```bash
    brew install jemalloc
    brew install pkg-config
    make install-deps
    ```

  - Ubuntu

    ```bash
    sudo apt-get install -y libjemalloc-dev libjemalloc2 pkg-config libbz2-dev
    make install-deps
    ```

- To ensure a successful build, you either need to synchronize the tags from the upstream repository or create a new tag.

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

## 📚 Documentation

You can find Juno's full documentation [here](https://juno.nethermind.io/).

If you are looking to become a **Starknet Validator**, follow our guide [here](https://nethermindeth.github.io/starknet-staking-v2/).

## 📸 Snapshots

Use the provided snapshots to quickly sync your Juno node with the current state of the network. Fresh snapshots are automatically uploaded once a week and are available under the links below.

#### Mainnet

| Version | Download Link |
| ------- | ------------- |
| **>=v0.13.0**  | [**juno_mainnet.tar**](https://juno-snapshots.nethermind.io/files/mainnet/latest) |

#### Sepolia

| Version | Download Link |
| ------- | ------------- |
| **>=v0.13.0** | [**juno_sepolia.tar**](https://juno-snapshots.nethermind.io/files/sepolia/latest) |

#### Sepolia-Integration

| Version | Download Link |
| ------- | ------------- |
| **>=v0.13.0** | [**juno_sepolia_integration.tar**](https://juno-snapshots.nethermind.io/files/sepolia-integration/latest) |

### Getting the size for each snapshot
```console
$date
Thu  1 Aug 2024 09:49:30 BST

$curl -s -I -L https://juno-snapshots.nethermind.io/files/mainnet/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
172.47 GB

$curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
5.67 GB

$curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia-integration/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
2.4 GB
```

### Run Juno Using Snapshot

1. **Download Snapshot**

   Fetch the snapshot from the provided URL:

   ```bash
   wget -O juno_mainnet.tar https://juno-snapshots.nethermind.io/files/mainnet/latest
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

- Starknet [v0.14.0](https://docs.starknet.io/resources/version-notes/#starknet_v0_14_0_july_28_25) support.
- JSON-RPC [v0.9.0-rc1](https://github.com/starkware-libs/starknet-specs/tree/v0.9.0-rc.1) (Available under `/v0_9` and default`/` endpoints)
  
  Chain/Network Information:
  - `starknet_chainId`
  - `starknet_specVersion`
  - `starknet_syncing`

  Block Related:
  - `starknet_blockNumber`
  - `starknet_blockHashAndNumber`
  - `starknet_getBlockWithTxHashes`
  - `starknet_getBlockWithTxs`
  - `starknet_getBlockWithReceipts`
  - `starknet_getBlockTransactionCount`

  Transaction Related:
  - `starknet_getTransactionByHash`
  - `starknet_getTransactionReceipt`
  - `starknet_getTransactionByBlockIdAndIndex`
  - `starknet_getTransactionStatus`
  - `starknet_getMessagesStatus`

  State & Storage:
  - `starknet_getNonce`
  - `starknet_getStorageAt`
  - `starknet_getStorageProof`
  - `starknet_getStateUpdate`

  Contract Related:
  - `starknet_getClassHashAt`
  - `starknet_getClass`
  - `starknet_getClassAt`
  - `starknet_getCompiledCasm`
  - `starknet_call`

  Event Related:
  - `starknet_getEvents`

  Write Operations:
  - `starknet_addInvokeTransaction`
  - `starknet_addDeclareTransaction`
  - `starknet_addDeployAccountTransaction`

  Fee Estimation & Simulation:
  - `starknet_estimateFee`
  - `starknet_estimateMessageFee`
  - `starknet_simulateTransactions`

  Tracing & Debug:
  - `starknet_traceTransaction`
  - `starknet_traceBlockTransactions`

  Websocket Subscriptions:
  - `starknet_subscribeNewHeads`
  - `starknet_subscribeEvents`
  - `starknet_subscribeTransactionStatus`
  - `starknet_subscribePendingTransactions`
  - `starknet_unsubscribe`

- Juno's JSON-RPC:
  - `juno_version`
- JSON-RPC [v0.9.0-rc1](https://github.com/starkware-libs/starknet-specs/tree/v0.9.0-rc.1) (Available under `/v0_9` endpoint)
- JSON-RPC [v0.8.0](https://github.com/starkware-libs/starknet-specs/releases/tag/v0.8.0) (Available under `/v0_8` endpoint)
- Integration of CairoVM. 
- Verification of State from L1.
- Handle L1 and L2 Reorgs.
- Starknet state construction and storage using a path-based Merkle Patricia trie.
- Feeder gateway synchronisation of Blocks, Transactions, Receipts, State Updates and Classes.
- Block and Transaction hash verification.
- Plugins 

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

For more details on how to get started, check out our [contributing guidelines](https://github.com/NethermindEth/juno/blob/main/CONTRIBUTING.md).

There are also many other ways to contribute. Here are some ideas:

* Run a node.
* Add a [GitHub Star](https://github.com/NethermindEth/juno/stargazers) to the project.
* [Tweet](https://twitter.com/intent/tweet?url=https%3A%2F%2Fgithub.com%2FNethermindEth%2Fjuno&via=nethermindeth&text=Juno%20is%20Awesome%2C%20they%20are%20working%20hard%20to%20bring%20decentralization%20to%20StarkNet&hashtags=StarkNet%2CJuno%2CEthereum) about Juno.
* Add a Github issue if you find a [bug](https://github.com/NethermindEth/juno/issues/new?assignees=&labels=&template=bug_report.md&title=), or you need or want a new [feature](https://github.com/NethermindEth/juno/issues/new?assignees=&labels=&template=feature_request.md&title=).

## 📞 Contact us

For questions or feedback, please don't hesitate to reach out to us:

- [Telegram](https://t.me/StarknetJuno)
- [Discord](https://discord.com/invite/TcHbSZ9ATd)
- [X(Formerly Twitter)](https://x.com/NethermindStark)



## 🤝 Partnerships

To establish a partnership with the Juno team, or if you have any suggestion or special request, feel free to reach us
via [email](mailto:juno@nethermind.io).

## ⚠️ License

Copyright (c) 2022-present, with the following [contributors](https://github.com/NethermindEth/juno/graphs/contributors).

Juno is open-source software licensed under the [Apache-2.0 License](https://github.com/NethermindEth/juno/blob/main/LICENSE).

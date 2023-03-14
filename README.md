<p align="center">
  <a href="https://github.com/NethermindEth/juno">
    <img alt="Juno Logo" height="125" src="./.github/juno_icon_light.png">
  </a>
  <br>
</p>

<h1 align="center">Juno</h1>

<p align="center">
  <a href="https://pkg.go.dev/github.com/NethermindEth/juno">
    <img src="https://pkg.go.dev/badge/github.com/NethermindEth/juno.svg">
  </a>
  <a href="https://goreportcard.com/report/github.com/NethermindEth/juno">
    <img src="https://goreportcard.com/badge/github.com/NethermindEth/juno">
  </a>
  <a href="https://github.com/NethermindEth/juno/actions">
    <img src="https://github.com/NethermindEth/juno/actions/workflows/juno-build.yml/badge.svg">
  </a>
  <a href="https://codecov.io/gh/NethermindEth/juno">
    <img src="https://codecov.io/gh/NethermindEth/juno/branch/main/graph/badge.svg">
  </a>

</p>
<p align="center">
  <a href="https://discord.gg/TcHbSZ9ATd">
    <img src="https://img.shields.io/badge/Discord-5865F2?style=for-the-badge&logo=discord&logoColor=white">
  </a>
  <a href="https://twitter.com/nethermindeth?s=20&t=xLC_xrid_f17DJqdJ2EZnA">
    <img src="https://img.shields.io/badge/Twitter-1DA1F2?style=for-the-badge&logo=twitter&logoColor=white">
  </a>
</p>


<p align="center">
  <b>Juno</b> is a golang <a href="https://starknet.io/">Starknet</a> node implementation by <a href="https://nethermind.io/">Nethermind</a> with the aim of decentralising Starknet.
</p>

## ⚙️ Installation

### Prerequisites

- Golang 1.19 or higher is required to build and run the project. You can find the installer on
  the official Golang [download](https://go.dev/doc/install) page.
- A C compiler: `gcc` or `clang`.

### Build and Run

```shell
make juno
./build/juno
```

### Run with Docker

```shell
docker run -it \
  -p 6060:6060 \
  -v /home/juno:/var/lib/juno \
  nethermindeth/juno \
  --rpc-port 6060 \
  --db-path /var/lib/juno \
  --verbosity 0 \
  --network 0
```

Before running the docker run command, please ensure that the directory `/home/juno` exists on your local machine.

## ✔ Supported Features

- Starknet state construction and storage using a path-based Merkle Patricia trie. 
- Pedersen and `starknet_keccak` hash implementation over starknet field.
- Feeder gateway synchronisation of Blocks, Transactions, Receipts, State Updates and Classes.
- Block and Transaction hash verification.
- JSON-RPC Endpoints:
  - `starknet_chainId`
  - `starknet_blockNumber`
  - `starknet_blockHashAndNumber`
  - `starknet_getBlockWithTxHashes`
  - `starknet_getBlockWithTxs`
  - `starknet_getTransactionByHash`
  - `starknet_getTransactionReceipt`
  - `starknet_getBlockTransactionCount`
  - `starknet_getTransactionByBlockIdAndIndex`
  - `starknet_getStateUpdate`

## 🛣 Roadmap

### Phase 1

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

### Phase 2

* [ ] Integrate cairo vm
* [ ] Verification
    * [ ] L1 verification
    * [ ] Execution of all transactions from feeder gateway
* [ ] Full RPC (according to 0.11.0)
* [ ] Start p2p discussions
* [ ] Infura and Alchemy integrations

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

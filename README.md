<p align="center">
  <a href="https://gojuno.xyz">
    <img alt="Juno Logo" height="125" src="./.github/juno_rounded.png">
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
  <b>Juno</b> is a golang <a href="https://starknet.io/">StarkNet</a> node implementation by <a href="https://nethermind.io/">Nethermind</a> with the aim of decentralising StarkNet.
</p>

## ⚙️ Installation

### Prerequisites

- Golang 1.18 or higher is required to build and run the project. You can find the installer on 
  the official Golang [download](https://go.dev/doc/install) page.
- A C compiler: `gcc` or `clang`.

### Build and Run

```shell
make juno
./build/juno
```


## 🛣 Roadmap

### Phase 1

* [X] Flat DB implementation of trie
* [X] Go implementation of crypto primitives
    * [X] Pedersen hash
    * [X] StarkNet_Keccak
    * [X] Felt
* [ ] Feeder gateway synchronisation (in progress)
    * [ ] State Update
    * [ ] Blocks
    * [ ] Transactions
    * [ ] Class
* [ ] Implement the following core data structures and their Hash calculations (in progress)
    * [ ] Blocks
    * [ ] Transactions and Transaction Receipts
    * [ ] Contracts and Classes
* [ ] Storing blocks, transactions, and State updates in local DB (in progress)
* [ ] Basic RPC (in progress)
    * [ ] `getBlockWithTxHashes`
    * [ ] `getBlockWithTxs`
    * [ ] `getBlockTransactionCount`
    * [ ] `getTransactionByHash`
    * [ ] `getTransactionByBlockIdAndIndex`

### Phase 2

* [ ] Integrate cairo rust-vm (discuss with lambda class, integrate starknet logic)
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

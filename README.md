<p align="center">
  <a href="https://gojuno.xyz">
    <img alt="Juno Logo" height="125" src="./docs/static/img/juno_rounded.png">
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
  <b>Juno</b> is a <a href="https://starknet.io/">StarkNet</a> node implementation written in <a href="https://go.dev/doc/">Golang</a> with ❤️ by <a href="https://nethermind.io/">Nethermind</a>. Designed to <b>ease</b> things up and <b>performance</b> in mind. We will bring decentralization to StarkNet.
</p>

## ⚙️ Installation

### Dependencies

- Golang 1.18 or higher is required to build and run the project. You can find the installer on the official Golang
  [download](https://go.dev/doc/install) page.
- Python 3.7

For a detailed installation guide, please refer to the [installation guide](https://gojuno.xyz/docs/intro#installing) 
page.

For details about the configuration you can look at
the [config file description](https://gojuno.xyz/docs/running/config).

### 📦 Dockerized

You can install the entire app using docker. Just check the
[docker guide](https://gojuno.xyz/docs/running/docker).

## 🎯 Features

- [Get and Sync state from Layer 1](https://gojuno.xyz/docs/features/sync) (Ethereum).
- [Get and Sync state from API](https://gojuno.xyz/docs/features/sync) (Feeder Gateway).
- Store [StarkNet State](https://gojuno.xyz/docs/features/sync) locally.
- Store StarkNet Transactions.
- Store StarkNet Blocks.
- Store the ABI and full code of StarkNet contracts.
- Ethereum-like [Json RPC Server](https://gojuno.xyz/docs/features/rpc) following
  [this spec](https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json). Currently
  supported are:
    - `starknet_getBlockWithTxHashes` - Get block information with transaction hashes given the block id.
     - `starknet_getBlockWithTxs` - Get block information with full transactions given the block id.
     - `starknet_getStateUpdate` - Get the information about the result of executing the requested block.
     - `starknet_getStorageAt` - Get the value of the storage at the given address and key.
     - `starknet_getTransactionByHash` - Get the details and status of a submitted transaction.
     - `starknet_getTransactionByBlockIdAndIndex` - Get the details of a transaction by a given block id and index.
     - `starknet_getTransactionReceipt` - Get the transaction receipt by the transaction hash.
     - `starknet_getClassHashAt` - Get the contract class hash in the given block for the contract deployed at the given address.
     - `starknet_getBlockTransactionCount` -  Get the number of transactions in a block given a block id.
     - `starknet_call` - Call a StarkNet function without creating a transaction.
     - `starknet_blockNumber` - Get the most recent accepted block number.
     - `starknet_blockHashAndNumber` - Get the most recent accepted block hash and number.
     - `starknet_chainId` - Return the currently configured StarkNet chain id.
     - `starknet_pendingTransactions` - Returns the transactions in the transaction pool, recognized by this sequencer.
     - `starknet_syncing` - Returns an object about the sync status, or false if the node is not synching.
- [Prometheus Metrics](https://gojuno.xyz/docs/features/metrics).
- [Dockerized app](https://gojuno.xyz/docs/running/docker).

## 🛣 Roadmap

In the future we plan to add a new set of features like:

- P2P between all nodes on the network.
- Complete support for v1 of the [RPC spec](https://github.com/starkware-libs/starknet-specs/releases/tag/v0.1.0)
- Faster sync
- Enhanced metrics
- And more! Stay tuned! 🚀

## 📜 Documentation

For further details, you can watch the [documentation](https://gojuno.xyz).

## 👍 Contribute

If you want to say **thank you** and/or support the active development of `Juno`:

1. Run a node.
2. Add a [GitHub Star](https://github.com/NethermindEth/juno/stargazers) to the project.
3. Tweet about
   `Juno` [on your Twitter](https://twitter.com/intent/tweet?url=https%3A%2F%2Fgithub.com%2FNethermindEth%2Fjuno&via=nethermindeth&text=Juno%20is%20Awesome%2C%20they%20are%20working%20hard%20to%20bring%20decentralization%20to%20StarkNet&hashtags=StarkNet%2CJuno%2CEthereum)
   .
4. Contribute to use, make sure to
   follow [Contributions Guidelines](https://gojuno.xyz/docs/contribution_guidelines/engineering-guidelines).
5. Add a Github issue if you find
   a [bug](https://github.com/NethermindEth/juno/issues/new?assignees=&labels=&template=bug_report.md&title=)
   , or you need or want a
   new [feature](https://github.com/NethermindEth/juno/issues/new?assignees=&labels=&template=feature_request.md&title=)
   .

## ‍💻 Code Contributors

<img src="./.github/contributors.svg" alt="Code Contributors" style="max-width:100%;">

## 🤝 Partnerships

To establish a partnership with the Juno team, or if you have any suggestion or special request, feel free to reach us
via [email](mailto:juno@nethermind.io).

## ⭐️ Stargazers over time

[![Stargazers over time](https://starchart.cc/NethermindEth/juno.svg)](https://starchart.cc/NethermindEth/juno)

## ⚠️ License

Copyright (c) 2022-present, with the following [contributors](https://github.com/NethermindEth/juno/graphs/contributors)
.
`Juno` is open-source software licensed under
the [Apache-2.0 License](https://github.com/NethermindEth/juno/blob/main/LICENSE).

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

Make sure you have Go installed ([download](https://go.dev/dl/)). Version `1.18` or higher is required.

At the time we write, we support two commands:

- juno
    - `juno` is the command that initialize the node
- juno-cli
    - `juno-cli` is the command that handle a set of different commands about the StarkNet ecosystem.

You can install `juno` command

```bash
go install github.com/NethermindEth/juno/cmd/juno@latest
```

On the other side, the `juno-cli` command can be installed with the next command:

```bash
go install github.com/NethermindEth/juno/cmd/juno-cli@latest
```

### 📦 Dockerized

You can install the app using docker, if you need it, can check the [guide](https://gojuno.xyz/docs/running/docker) we
made for that

## 🎯 Features

- [Get and Sync state from Layer 1](https://gojuno.xyz/docs/features/sync) (Ethereum).
- [Get and Sync state from API](https://gojuno.xyz/docs/features/sync) (Feeder Gateway)
- Hold [state of StarkNet](https://gojuno.xyz/docs/features/sync) locally.
- Hold the StarkNet Transactions.
- Hold the StarkNet Blocks.
- Hold the ABI of StarkNet contracts.
- [Json RPC Server](https://gojuno.xyz/docs/features/rpc) ethereum like following
  [this spec](https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json). Now
  supported:
    - starknet_getStorageAt
    - starknet_getCode
    - starknet_getBlockByHash
    - starknet_getBlockByNumber
    - starknet_getTransactionByHash
    - starknet_getTransactionByBlockHashAndIndex
    - starknet_getStorageAt (pending)
    - starknet_getCode (pending)
    - starknet_getBlockByNumber (pending)
- [Rest API](https://gojuno.xyz/docs/features/rest) is a wrapper of the StarkNet feeder gateway, you can call the node
  in the same way you call the feeder gateway, same params and should return the same response.
- [CLI](https://gojuno.xyz/docs/features/cli) for the StarkNet tools
- [Prometheus Metrics](https://gojuno.xyz/docs/feature/metrics).
- [Dockerized app](https://gojuno.xyz/docs/running/docker).

## 📜 Documentation

For further details, you can watch the [documentation](https://gojuno.xyz). 

## 👍 Contribute

If you want to say **thank you** and/or support the active development of `Juno`:

1. Run a node.
2. Add a [GitHub Star](https://github.com/NethermindEth/juno/stargazers) to the project.
3. Tweet about the
   `Juno` [on your Twitter](https://twitter.com/intent/tweet?url=https%3A%2F%2Fgithub.com%2FNethermindEth%2Fjuno&via=nethermindeth&text=Juno%20is%20Awesome%2C%20they%20are%20working%20hard%20to%20bring%20decentralization%20to%20StarkNet&hashtags=StarkNet%2CJuno%2CEthereum)
   .
4. Contribute to use, make sure to
   follow [Contributions Guidelines](https://gojuno.xyz/docs/contribution_guidelines/engineering-guidelines)
5. Add an issue if you find
   a [bug](https://github.com/NethermindEth/juno/issues/new?assignees=&labels=&template=bug_report.md&title=)
   , or you need or want a
   new [feature](https://github.com/NethermindEth/juno/issues/new?assignees=&labels=&template=feature_request.md&title=)

## ‍💻 Code Contributors

<img src="./.github/contributors.svg" alt="Code Contributors" style="max-width:100%;">

## ⭐️ Stargazers over time

[![Stargazers over time](https://starchart.cc/NethermindEth/juno.svg)](https://starchart.cc/NethermindEth/juno)

## ⚠️ License

Copyright (c) 2022-present, with the following [contributors](https://github.com/NethermindEth/juno/graphs/contributors)
.
`Juno` is open-source software licensed under
the [Apache-2.0 License](https://github.com/NethermindEth/juno/blob/main/LICENSE).
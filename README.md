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
  <b>Juno</b> is a <a href="https://starknet.io/">StarkNet</a> node implementation written in <a href="https://go.dev/doc/">Golang</a> by <a href="https://nethermind.io/">Nethermind</a> with the aim of decentralising StarkNet.
</p>

## ⚙️ Installation

### Dependencies

- Golang 1.18 or higher is required to build and run the project. You can find the installer on the official Golang
  [download](https://go.dev/doc/install) page.
- Python 3.7

```
make juno
```
## 🛣 Roadmap

In the future we plan to add a new set of features like:

- [Get and Sync state from Layer 1](https://gojuno.xyz/docs/features/sync) (Ethereum).
- [Get and Sync state from API](https://gojuno.xyz/docs/features/sync) (Feeder Gateway).
- Store [StarkNet State](https://gojuno.xyz/docs/features/sync) locally.
- Store StarkNet Transactions.
- Store StarkNet Blocks.
- Store the ABI and full code of StarkNet contracts.
- Ethereum-like [Json RPC Server](https://gojuno.xyz/docs/features/rpc) following
  [the v0.1.0 spec](https://github.com/starkware-libs/starknet-specs/blob/v0.1.0/api/starknet_api_openrpc.json).
- [Prometheus Metrics](https://gojuno.xyz/docs/features/metrics).
- [Dockerized app](https://gojuno.xyz/docs/running/docker).
- P2P between all nodes on the network.
- Complete support for v0.2.0 of the [RPC spec](https://github.com/starkware-libs/starknet-specs/releases/tag/v0.1.0)
- Faster sync
- Enhanced metrics
- And more! Stay tuned! 🚀

## 👍 Contribute

If you want to say **thank you** and/or support the active development of `Juno`:

1. Run a node.
2. Add a [GitHub Star](https://github.com/NethermindEth/juno/stargazers) to the project.
3. Tweet about`Juno` [on your Twitter](https://twitter.com/intent/tweet?url=https%3A%2F%2Fgithub.com%2FNethermindEth%2Fjuno&via=nethermindeth&text=Juno%20is%20Awesome%2C%20they%20are%20working%20hard%20to%20bring%20decentralization%20to%20StarkNet&hashtags=StarkNet%2CJuno%2CEthereum).
4. Add a Github issue if you find a [bug](https://github.com/NethermindEth/juno/issues/new?assignees=&labels=&template=bug_report.md&title=), or you need or want a new [feature](https://github.com/NethermindEth/juno/issues/new?assignees=&labels=&template=feature_request.md&title=).
5. Add Pull Request for a new feature.

## 🤝 Partnerships

To establish a partnership with the Juno team, or if you have any suggestion or special request, feel free to reach us
via [email](mailto:juno@nethermind.io).

## ⚠️ License

Copyright (c) 2022-present, with the following [contributors](https://github.com/NethermindEth/juno/graphs/contributors)
.
`Juno` is open-source software licensed under
the [Apache-2.0 License](https://github.com/NethermindEth/juno/blob/main/LICENSE).

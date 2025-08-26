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


## 📚 Documentation

To know everything there is to know on how to interact with Juno, please check out the [documentation](https://juno.nethermind.io/). 

To become a **Starknet Validator** follow this [guide](https://nethermindeth.github.io/starknet-staking-v2/).

## ⚙️ Installation

### Prerequisites

- Golang 1.25 or higher is required to build and run the project. You can find the installer on
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

## 📸 Starknet Snapshots

To sync your node as fast as possible, use the provided snapshots service. Fresh snapshots are automatically uploaded once a week. See [Database Snapshots](https://juno.nethermind.io/snapshots) to get instructions on how to set Juno up and running using a recent Starknet snapshot.

## 🌐 Interacting with Juno

Juno works as gateway into Starknet and can be queried using JSON-RPC requests. Also, it allows to subscribe to real time events happening on the network and get constant updates. Read the [Interacting with Juno] section for more details.


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

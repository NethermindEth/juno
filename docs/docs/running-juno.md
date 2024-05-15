---
title: Running Juno
---

# Running Juno :rocket:

You can run a Juno node using several methods:

- [Docker container](#docker-container)
- [Standalone binary](#standalone-binary)
- [Building from source](#building-from-source)
- [Google Cloud Platform (GCP)](running-on-gcp)

:::tip
You can use a snapshot for fast synchronisation with the Starknet Mainnet/Sepolia networks. Check out the [Database Snapshots](snapshots) guide to get started.
:::

## Docker container

### 1. Get the Docker image

Juno Docker images can be found at the [nethermind/juno](https://hub.docker.com/r/nethermind/juno) repository on Docker Hub. Download the latest image:

```bash
docker pull nethermind/juno
```

You can also build the image locally:

```bash
# Clone the Juno repository
git clone https://github.com/NethermindEth/juno
cd juno

# Build the Docker image
docker build -t nethermind/juno:latest .
```

### 2. Run the Docker container

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --eth-node <YOUR ETH NODE>
```

:::info
Replace \<YOUR ETH NODE\> with the WebSocket endpoint of your Ethereum node. For Infura users, your address should be: `wss://mainnet.infura.io/ws/v3/your-infura-project-id`. Ensure you use the WebSocket URL (`ws`/`wss`) instead of the HTTP URL (`http`/`https`).
:::

You can view logs from the Docker container using the following command:

```bash
docker logs -f juno
```

## Standalone binary

Download standalone binaries from [Juno's GitHub Releases](https://github.com/NethermindEth/juno/tags) as ZIP archives for Linux (amd64 and arm64) and macOS (amd64).

For macOS (arm64) or Windows users, consider [running Juno using Docker](#docker-container).

## Building from source

You can build the Juno binary or Docker image from the source code to access the latest updates or specific versions.

### Prerequisites

- [Golang 1.22](https://go.dev/doc/install) or later
- [Rust](https://www.rust-lang.org/tools/install)
- C compiler: `gcc` or `clang`
- [jemalloc](https://github.com/jemalloc/jemalloc)

```mdx-code-block
import Tabs from "@theme/Tabs";
import TabItem from "@theme/TabItem";
```

<Tabs>
<TabItem value="ubuntu" label="Ubuntu">

```bash
sudo apt-get install -y libjemalloc-dev
```

</TabItem>
<TabItem value="mac" label="MacOS (Homebrew)">

```bash
brew install jemalloc
```

</TabItem>
</Tabs>

### 1. Clone the repository

Clone Juno's source code from our [GitHub repository](https://github.com/NethermindEth/juno):

```bash
git clone https://github.com/NethermindEth/juno
cd juno
```

:::tip
You can use `git tag -l` to view specific version tags.
:::

### 2. Build the binary or Docker image

```bash
# Build the binary
make juno

# Build the Docker image
docker build -t nethermind/juno:latest .
```

### 3. Run the binary

Locate the standalone binary in the `./build/` directory:

```bash
# Run with the default configuration
./build/juno

# See the available command line options
./build/juno --help
```

:::tip
To learn how to configure Juno, check out the [Configuring Juno](configuring) guide.
:::

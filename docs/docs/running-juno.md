---
title: Running Juno
---

# Running Juno :rocket:

```mdx-code-block
import Tabs from "@theme/Tabs";
import TabItem from "@theme/TabItem";
```

You can run a Juno node using several methods:

- **Docker container** or **Standalone binary** — see below
- [Building from source](#building-from-source)
- [Kubernetes with Helm](running-on-kubernetes)
- [Google Cloud Platform (GCP)](running-on-gcp)

:::tip
You can use a snapshot to quickly synchronise your node with the network. Check out the [Database Snapshots](snapshots) guide to get started.
:::

<Tabs groupId="install">
<TabItem value="docker" label="Docker" default>

**1. Get the Docker image**

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

**2. Run the Docker container**

```bash
# Prepare the snapshots directory
mkdir -p $HOME/snapshots

# Run the container
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/snapshots/juno_mainnet:/snapshots/juno_mainnet \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --eth-node <YOUR-ETH-NODE> \
  --db-path /snapshots/juno_mainnet
```

You can view logs from the Docker container using the following command:

```bash
docker logs -f juno
```

</TabItem>
<TabItem value="binary" label="Standalone Binary">

**1. Download the binary**

Download the standalone binary from [Juno's GitHub Releases](https://github.com/NethermindEth/juno/releases/latest) as a ZIP archive for Linux and MacOS (amd64 and arm64). For Windows users, consider running Juno via **Docker** instead.

**2. Run the binary**

```bash
# Prepare the snapshots directory
mkdir -p $HOME/snapshots

# Run the binary
./juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --eth-node <YOUR-ETH-NODE> \
  --db-path $HOME/snapshots/juno_mainnet
```

Replace `<YOUR-ETH-NODE>` with your actual Ethereum node address. If you're using Infura, it might look something like `wss://mainnet.infura.io/ws/v3/your-infura-project-id`. Make sure you use the WebSockets URL (`ws`/`wss`) and not the HTTP URL (`http`/`https`).

You can view logs from the standalone binary by redirecting the output to a file:

```shell
./juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --eth-node <YOUR-ETH-NODE> \
  --db-path $HOME/snapshots/juno_mainnet \
  > juno.log 2>&1
```

</TabItem>
</Tabs>

## Building from source

You can build the Juno binary or Docker image from the source code to access the latest updates or specific versions.

<Tabs groupId="install">
<TabItem value="docker" label="Docker" default>

Building the Docker image requires only [Docker](https://docs.docker.com/get-docker/).

**1. Clone the repository**

```bash
git clone https://github.com/NethermindEth/juno
cd juno
```

:::tip
You can use `git tag -l` to view specific version tags.
:::

**2. Build the Docker image**

```bash
docker build -t nethermind/juno:latest .
```

**3. Run the Docker container**

```bash
# Prepare the snapshots directory
mkdir -p $HOME/snapshots

# Run the container
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/snapshots/juno_mainnet:/snapshots/juno_mainnet \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --eth-node <YOUR-ETH-NODE> \
  --db-path /snapshots/juno_mainnet
```

</TabItem>
<TabItem value="binary" label="Standalone Binary">

**Prerequisites**

- [Golang 1.26](https://go.dev/doc/install) or later
- [Rust](https://www.rust-lang.org/tools/install) 1.88.0 or higher.
- C compiler: `gcc` or `clang`
- [jemalloc](https://github.com/jemalloc/jemalloc)

<Tabs>
<TabItem value="ubuntu" label="Ubuntu">

```bash
sudo apt-get install -y build-essential make libjemalloc-dev libjemalloc2 pkg-config libbz2-dev
```

</TabItem>
<TabItem value="mac" label="MacOS (Homebrew)">

```bash
brew install jemalloc pkg-config
```

</TabItem>
</Tabs>

**1. Clone the repository**

```bash
git clone https://github.com/NethermindEth/juno
cd juno
```

:::tip
You can use `git tag -l` to view specific version tags.
:::

**2. Build the binary**

```bash
# Install juno dependencies
make install-deps

# Build the binary
make juno
```

**3. Run the binary**

Locate the standalone binary in the `./build/` directory:

```bash
# Prepare the snapshots directory
mkdir -p $HOME/snapshots

# Run the binary
./build/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --db-path $HOME/snapshots/juno_mainnet \
  --eth-node <YOUR-ETH-NODE>
```

</TabItem>
</Tabs>

:::tip
To learn how to configure Juno, check out the [Configuring Juno](configuring) guide.
:::

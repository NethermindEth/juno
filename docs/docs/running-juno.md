---
title: Running Juno
---

# Running Juno :rocket:

You can run a Juno node using several methods:

- [Docker container](#docker-container) (recommended)
- [Standalone binaries](#standalone-binaries)
- [Building from source](#building-from-source)
- [Google Cloud Platform (GCP)](running-on-gcp)

:::tip

- You can use a snapshot for fast synchronisation with the Starknet Mainnet/Sepolia networks. Check out the [Database Snapshots](snapshots) guide to get started.
- You can access the Nethermind Starknet RPC service for free at https://data.voyager.online.

:::

## Docker container

#### 1. Get the Docker image

The Juno Docker images are available on Docker Hub at the [nethermind/juno](https://hub.docker.com/r/nethermind/juno) repository. Pull the latest image:

```bash
docker pull nethermind/juno
```

You can also build the image locally:

```bash
# Clone the repository
git clone https://github.com/NethermindEth/juno
cd juno

# Build Docker image
docker build -t nethermind/juno:latest .
```

#### 2. Run the Docker container

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
Replace \<YOUR ETH NODE\> with the WebSocket endpoint of your Ethereum node. For Infura users, your address should resemble: `wss://mainnet.infura.io/ws/v3/your-infura-project-id`. Ensure you use the WebSocket URL (`ws`/`wss`) instead of the HTTP URL (`http`/`https`).
:::

You can view logs from the Docker container using the following command:

```bash
docker logs -f juno
```

## Standalone binaries

Standalone binaries are available on [GitHub Releases](https://github.com/NethermindEth/juno/tags) as ZIP archives for Linux (amd64 and arm64) and macOS (amd64).

:::info
If you wish to run Juno on macOS (arm64) or Windows, consider using a [Docker container](#docker-container) or [building from source](#building-from-source).
:::

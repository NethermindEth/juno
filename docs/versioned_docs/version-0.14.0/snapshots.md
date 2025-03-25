---
title: Database Snapshots
---

# Database Snapshots :camera_flash:

You can download a snapshot of the Juno database to reduce the network syncing time. Only the blocks created after the snapshot will be synced when you run the node. Fresh snapshots are automatically uploaded once a week and are available under the links below.

## Mainnet

| Version | Download Link |
| ------- | ------------- |
| **>=v0.13.0**  | [**juno_mainnet.tar**](https://juno-snapshots.nethermind.io/files/mainnet/latest) |

## Sepolia

| Version | Download Link |
| ------- | ------------- |
| **>=v0.13.0** | [**juno_sepolia.tar**](https://juno-snapshots.nethermind.io/files/sepolia/latest) |

## Sepolia-Integration

| Version | Download Link |
| ------- | ------------- |
| **>=v0.13.0** | [**juno_sepolia_integration.tar**](https://juno-snapshots.nethermind.io/files/sepolia-integration/latest) |

## Getting snapshot sizes

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

## Run Juno with a snapshot

### 1. Download the snapshot

First, download a snapshot from one of the provided URLs:

```bash
wget -O juno_mainnet.tar https://juno-snapshots.nethermind.io/files/mainnet/latest
```

### 2. Prepare a directory

Ensure you have a directory to store the snapshots. We will use the `$HOME/snapshots` directory:

```bash
mkdir -p $HOME/snapshots
```

### 3. Extract the snapshot

Extract the contents of the downloaded `.tar` file into the directory:

```bash
tar -xvf juno_mainnet.tar -C $HOME/snapshots
```

### 4. Run Juno

Run the Docker command to start Juno and provide the path to the snapshot using the `db-path` option:

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/snapshots/juno_mainnet:/snapshots/juno_mainnet \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --db-path /snapshots/juno_mainnet \
  --eth-node <YOUR ETH NODE>
```

:::info
Replace \<YOUR ETH NODE\> with the WebSocket endpoint of your Ethereum node. For Infura users, your address should be: `wss://mainnet.infura.io/ws/v3/your-infura-project-id`. Ensure you use the WebSocket URL (`ws`/`wss`) instead of the HTTP URL (`http`/`https`).
:::

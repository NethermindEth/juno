---
title: Database Snapshots
---

# Database Snapshots :camera_flash:

You can download a snapshot of the Juno database to reduce the network syncing time. Only the blocks created after the snapshot will be synced when you run the node. Fresh snapshots are automatically uploaded once a week and are available under the links below.

**Note**: Snapshots are now provided in compressed `.tar.zst` format for faster downloads and reduced storage requirements. You can stream the download and extraction process without requiring double disk space.

## Mainnet

| Version | Download Link |
| ------- | ------------- |
| **>=v0.13.0**  | [**juno_mainnet.tar.zst**](https://juno-snapshots.nethermind.io/files/mainnet/latest) |

## Sepolia

| Version | Download Link |
| ------- | ------------- |
| **>=v0.13.0** | [**juno_sepolia.tar.zst**](https://juno-snapshots.nethermind.io/files/sepolia/latest) |

## Sepolia-Integration

| Version | Download Link |
| ------- | ------------- |
| **>=v0.13.0** | [**juno_sepolia_integration.tar.zst**](https://juno-snapshots.nethermind.io/files/sepolia-integration/latest) |

## Getting snapshot sizes

```console
$date
Mon 28 Jul 2025 08:24:59 GMT

$curl -s -I -L https://juno-snapshots.nethermind.io/files/mainnet/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
186.23 GB

$curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
30.45 GB

$curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia-integration/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
6.14 GB
```

## Run Juno with a snapshot

This method downloads and extracts the snapshot in one step without requiring double disk space:

### 1. Prepare a directory

Ensure you have a directory to store the snapshots. We will use the `$HOME/snapshots` directory:

```bash
mkdir -p $HOME/snapshots
```

### 2. Install zstd

[zstd (Zstandard)](https://github.com/facebook/zstd) is required to decompress and directly stream the snapshots into your system without requiring temporary storage. zstd provides significantly better compression ratios and faster decompression speeds compared to traditional tar compression.

```bash
# On Ubuntu/Debian
sudo apt-get install zstd

# On macOS
brew install zstd

# On RHEL/CentOS/Fedora
sudo dnf install zstd  # or yum install zstd
```

### 3. Stream download and extract

Download and extract the snapshot directly to your target directory:

```bash
# For Mainnet
curl -s -L https://juno-snapshots.nethermind.io/files/mainnet/latest \
| zstd -d | tar -xvf - -C $HOME/snapshots
```

For other networks, replace the URL with:
- **Sepolia**: `https://juno-snapshots.nethermind.io/files/sepolia/latest`
- **Sepolia-Integration**: `https://juno-snapshots.nethermind.io/files/sepolia-integration/latest`

#### Alternative method: Download then extract

If you prefer the traditional two-step approach or have limited bandwidth, you can download the snapshot first and extract it later:

##### 1. Download the snapshot

```bash
# For Mainnet
wget -O juno_mainnet.tar.zst https://juno-snapshots.nethermind.io/files/mainnet/latest

# Or using curl
curl -L https://juno-snapshots.nethermind.io/files/mainnet/latest -o juno_mainnet.tar.zst
```

##### 2. Extract the snapshot

```bash
# Extract to your snapshots directory
zstd -d juno_mainnet.tar.zst -c | tar -xvf - -C $HOME/snapshots
```

## Running Juno with snapshots

### 1. Run Juno

Run the Docker command to start Juno:

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/snapshots:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --eth-node <YOUR-ETH-NODE>
```

:::info
Replace `<YOUR-ETH-NODE>` with your Ethereum node WebSocket URL (e.g., `wss://mainnet.infura.io/ws/v3/your-project-id`). Ensure you use the WebSocket URL (`ws`/`wss`) instead of the HTTP URL (`http`/`https`).
:::
---
title: Database Snapshots
---

# Database Snapshots :camera_flash:

You can download a snapshot of the Juno database to reduce the network syncing time. Only the blocks created after the snapshot will be synced when you run the node. Fresh snapshots are automatically uploaded once a week and are available under the links below.

Snapshots are provided in a compressed `.tar.zst` format for faster downloads and reduced storage requirements. It also allows you to directly stream the decompressed file to your computer without needing to download it first.

| Network             | Download Link                                                                                                 |
| ------------------- | ------------------------------------------------------------------------------------------------------------- |
| Mainnet             | [**juno_mainnet.tar.zst**](https://juno-snapshots.nethermind.io/files/mainnet/latest)                         |
| Mainnet-Pruned      | [**juno_mainnet_pruned.tar.zst**](https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest)           |
| Sepolia             | [**juno_sepolia.tar.zst**](https://juno-snapshots.nethermind.io/files/sepolia/latest)                         |
| Sepolia-Integration | [**juno_sepolia_integration.tar.zst**](https://juno-snapshots.nethermind.io/files/sepolia-integration/latest) |

```mdx-code-block
import Tabs from "@theme/Tabs";
import TabItem from "@theme/TabItem";
```

:::tip
Select your network in any tab below and the rest of the page follows — the choice is synced across every command on this page.
:::

## Getting snapshot sizes

Snapshot sizes are refreshed weekly. As of `Tue Jun 23 2026`:

<Tabs groupId="network">
<TabItem value="mainnet" label="Mainnet" default>

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/mainnet/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 433.68 GB
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet-Pruned">

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 76.75 GB
```

</TabItem>
<TabItem value="sepolia" label="Sepolia">

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 72.88 GB
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia-integration/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 35.12 GB
```

</TabItem>
</Tabs>

## Run Juno with a snapshot

This method downloads and extracts the snapshot in one step without requiring double disk space:

### 1. Prepare a directory

Ensure you have a directory to store the snapshots. We will use the `$HOME/snapshots` directory:

```bash
mkdir -p $HOME/snapshots
```

### 2. Install zstd

[zstd (Zstandard)](https://github.com/facebook/zstd) is required to decompress and directly stream the snapshots into your system without requiring temporary storage. `zstd` provides significantly better compression ratios and faster decompression speeds compared to traditional tar compression.

<Tabs groupId="os">
<TabItem value="ubuntu" label="Ubuntu/Debian" default>

```bash
sudo apt-get install zstd
```

</TabItem>
<TabItem value="macos" label="macOS">

```bash
brew install zstd
```

</TabItem>
<TabItem value="rhel" label="RHEL/CentOS/Fedora">

```bash
sudo dnf install zstd  # or yum install zstd
```

</TabItem>
</Tabs>

### 3. Download and extract

Two-step approach where we first download the snapshot and extract it later. Note that this will create the requirement to have twice the space required for the Juno snapshot. If space is not enough you can always try the **alternative method** below.

##### 1. Download the snapshot

<Tabs groupId="network">
<TabItem value="mainnet" label="Mainnet" default>

```bash
wget --continue -O "$HOME/snapshots/juno_mainnet.tar.zst" https://juno-snapshots.nethermind.io/files/mainnet/latest
```

Or using `curl`:

```bash
curl -L -C - -o $HOME/snapshots/juno_mainnet.tar.zst https://juno-snapshots.nethermind.io/files/mainnet/latest
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet-Pruned">

```bash
wget --continue -O "$HOME/snapshots/juno_mainnet_pruned.tar.zst" https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest
```

Or using `curl`:

```bash
curl -L -C - -o $HOME/snapshots/juno_mainnet_pruned.tar.zst https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest
```

</TabItem>
<TabItem value="sepolia" label="Sepolia">

```bash
wget --continue -O "$HOME/snapshots/juno_sepolia.tar.zst" https://juno-snapshots.nethermind.io/files/sepolia/latest
```

Or using `curl`:

```bash
curl -L -C - -o $HOME/snapshots/juno_sepolia.tar.zst https://juno-snapshots.nethermind.io/files/sepolia/latest
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
wget --continue -O "$HOME/snapshots/juno_sepolia_integration.tar.zst" https://juno-snapshots.nethermind.io/files/sepolia-integration/latest
```

Or using `curl`:

```bash
curl -L -C - -o $HOME/snapshots/juno_sepolia_integration.tar.zst https://juno-snapshots.nethermind.io/files/sepolia-integration/latest
```

</TabItem>
</Tabs>

##### 2. Extract the snapshot

Create a subfolder inside `$HOME/snapshots` where to unzip the downloaded snapshot:

<Tabs groupId="network">
<TabItem value="mainnet" label="Mainnet" default>

```bash
mkdir $HOME/snapshots/mainnet/
```

```bash
# Extract to your snapshots directory
zstd -d juno_mainnet.tar.zst -c | tar -xvf - -C $HOME/snapshots/mainnet
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet-Pruned">

```bash
mkdir $HOME/snapshots/mainnet-pruned/
```

```bash
# Extract to your snapshots directory
zstd -d juno_mainnet_pruned.tar.zst -c | tar -xvf - -C $HOME/snapshots/mainnet-pruned
```

</TabItem>
<TabItem value="sepolia" label="Sepolia">

```bash
mkdir $HOME/snapshots/sepolia/
```

```bash
# Extract to your snapshots directory
zstd -d juno_sepolia.tar.zst -c | tar -xvf - -C $HOME/snapshots/sepolia
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
mkdir $HOME/snapshots/sepolia-integration/
```

```bash
# Extract to your snapshots directory
zstd -d juno_sepolia_integration.tar.zst -c | tar -xvf - -C $HOME/snapshots/sepolia-integration
```

</TabItem>
</Tabs>

#### Alternative method: Stream the snapshot directly

:::warning
Streaming can become unreliable if the network conditions are not extremely good, requiring multiple restarts. Resort to this if disk space is at a premium.
:::

Create a subfolder inside `$HOME/snapshots` where to stream the download, then download and extract the snapshot directly to your target directory:

<Tabs groupId="network">
<TabItem value="mainnet" label="Mainnet" default>

```bash
mkdir $HOME/snapshots/mainnet/
```

```bash
curl -s -L https://juno-snapshots.nethermind.io/files/mainnet/latest \
| zstd -d | tar -xvf - -C $HOME/snapshots/mainnet
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet-Pruned">

```bash
mkdir $HOME/snapshots/mainnet-pruned/
```

```bash
curl -s -L https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest \
| zstd -d | tar -xvf - -C $HOME/snapshots/mainnet-pruned
```

</TabItem>
<TabItem value="sepolia" label="Sepolia">

```bash
mkdir $HOME/snapshots/sepolia/
```

```bash
curl -s -L https://juno-snapshots.nethermind.io/files/sepolia/latest \
| zstd -d | tar -xvf - -C $HOME/snapshots/sepolia
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
mkdir $HOME/snapshots/sepolia-integration/
```

```bash
curl -s -L https://juno-snapshots.nethermind.io/files/sepolia-integration/latest \
| zstd -d | tar -xvf - -C $HOME/snapshots/sepolia-integration
```

</TabItem>
</Tabs>

## Running Juno with snapshots

### 1. Run Juno

Run the Docker command to start Juno:

<Tabs groupId="network">
<TabItem value="mainnet" label="Mainnet" default>

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/snapshots/mainnet:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --eth-node <YOUR-ETH-NODE>
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet-Pruned">

A pruned snapshot is a Mainnet database with historical state and block data removed. Run it with `--prune-mode` enabled so the database stays pruned as the node keeps syncing; without it the node would begin retaining full history again and the database would grow back to the full Mainnet size.

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/snapshots/mainnet-pruned:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --prune-mode \
  --eth-node <YOUR-ETH-NODE>
```

</TabItem>
<TabItem value="sepolia" label="Sepolia">

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/snapshots/sepolia:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --network sepolia \
  --eth-node <YOUR-ETH-NODE>
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/snapshots/sepolia-integration:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --network sepolia-integration \
  --eth-node <YOUR-ETH-NODE>
```

</TabItem>
</Tabs>

:::info
Replace `<YOUR-ETH-NODE>` with your Ethereum node WebSocket URL, and make sure it matches the network's L1: Starknet Mainnet settles on Ethereum Mainnet (e.g. `wss://mainnet.infura.io/ws/v3/your-project-id`), while Sepolia and Sepolia-Integration settle on Ethereum Sepolia (e.g. `wss://sepolia.infura.io/ws/v3/your-project-id`). Ensure you use the WebSocket URL (`ws`/`wss`) instead of the HTTP URL (`http`/`https`).
:::

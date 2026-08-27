---
title: Sync from a Snapshot
---

# Sync from a Snapshot :camera_flash:

It is possible to avoid syncing from the beginning and waiting weeks to catch up by downloading a Juno snapshot. You're downloading a pre-synced Juno database that you can point your node to. This will reduce the syncing time to just a few hours.

Snapshots are provided in a compressed `.tar.zst` format for faster downloads and reduced storage requirements. It also allows you to directly stream the decompressed file to your computer without needing to download it first.

Additionally, _pruned_ snapshots are offered — they contain only the latest data, greatly reducing storage size.


## Network Snapshots

| Network             | Download Link                                                                                                 |
| ------------------- | ------------------------------------------------------------------------------------------------------------- |
| Mainnet             | [**juno_mainnet.tar.zst**](https://juno-snapshots.nethermind.io/files/mainnet/latest)                         |
| Mainnet (Pruned)    | [**juno_mainnet_pruned.tar.zst**](https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest)           |
| Sepolia             | [**juno_sepolia.tar.zst**](https://juno-snapshots.nethermind.io/files/sepolia/latest)                         |
| Sepolia (Pruned)    | [**juno_sepolia_pruned.tar.zst**](https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest)           |
| Sepolia-Integration | [**juno_sepolia_integration.tar.zst**](https://juno-snapshots.nethermind.io/files/sepolia-integration/latest) |

```mdx-code-block
import Tabs from "@theme/Tabs";
import TabItem from "@theme/TabItem";
```

:::tip
Select your network in any tab below and the rest of the page follows — the choice is synced across every command on this page.
:::

## Getting snapshot sizes

Snapshot sizes as of `Fri Jul 31 2026`:

<Tabs groupId="network">
<TabItem value="mainnet" label="Mainnet" default>

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/mainnet/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 453.93 GB
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet (Pruned)">

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 85.63 GB
```

</TabItem>
<TabItem value="sepolia" label="Sepolia">

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 77.14 GB
```

</TabItem>
<TabItem value="sepolia-pruned" label="Sepolia (Pruned)">

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 23.56 GB
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia-integration/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 38.64 GB
```

</TabItem>
</Tabs>

## Run Juno with a snapshot

You can either download the snapshot archive and then extract it, or stream it directly into the target directory without storing the archive. Both methods share the first two steps:

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
sudo dnf install zstd
```

</TabItem>
</Tabs>

### 3. Get the snapshot

<Tabs groupId="snapshot-method">
<TabItem value="download-extract" label="Download and then extract" default>

Two-step approach where we first download the snapshot and extract it later. Note that this will create the requirement to have twice the space required for the Juno snapshot. If space is not enough, use the **Streaming** tab instead.

##### 1. Download the snapshot

Both `wget --continue` and `curl -C -` resume an interrupted download: if the transfer dies for any reason, re-run the same command and it continues from where it stopped.

<Tabs groupId="network">
<TabItem value="mainnet" label="Mainnet" default>

<Tabs groupId="download-tool">
<TabItem value="wget" label="wget" default>

```bash
wget --continue -O "$HOME/snapshots/juno_mainnet.tar.zst" https://juno-snapshots.nethermind.io/files/mainnet/latest
```

</TabItem>
<TabItem value="curl" label="curl">

```bash
curl -L -C - -o $HOME/snapshots/juno_mainnet.tar.zst https://juno-snapshots.nethermind.io/files/mainnet/latest
```

</TabItem>
</Tabs>

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet (Pruned)">

<Tabs groupId="download-tool">
<TabItem value="wget" label="wget" default>

```bash
wget --continue -O "$HOME/snapshots/juno_mainnet_pruned.tar.zst" https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest
```

</TabItem>
<TabItem value="curl" label="curl">

```bash
curl -L -C - -o $HOME/snapshots/juno_mainnet_pruned.tar.zst https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest
```

</TabItem>
</Tabs>

</TabItem>
<TabItem value="sepolia" label="Sepolia">

<Tabs groupId="download-tool">
<TabItem value="wget" label="wget" default>

```bash
wget --continue -O "$HOME/snapshots/juno_sepolia.tar.zst" https://juno-snapshots.nethermind.io/files/sepolia/latest
```

</TabItem>
<TabItem value="curl" label="curl">

```bash
curl -L -C - -o $HOME/snapshots/juno_sepolia.tar.zst https://juno-snapshots.nethermind.io/files/sepolia/latest
```

</TabItem>
</Tabs>

</TabItem>
<TabItem value="sepolia-pruned" label="Sepolia (Pruned)">

<Tabs groupId="download-tool">
<TabItem value="wget" label="wget" default>

```bash
wget --continue -O "$HOME/snapshots/juno_sepolia_pruned.tar.zst" https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest
```

</TabItem>
<TabItem value="curl" label="curl">

```bash
curl -L -C - -o $HOME/snapshots/juno_sepolia_pruned.tar.zst https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest
```

</TabItem>
</Tabs>

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

<Tabs groupId="download-tool">
<TabItem value="wget" label="wget" default>

```bash
wget --continue -O "$HOME/snapshots/juno_sepolia_integration.tar.zst" https://juno-snapshots.nethermind.io/files/sepolia-integration/latest
```

</TabItem>
<TabItem value="curl" label="curl">

```bash
curl -L -C - -o $HOME/snapshots/juno_sepolia_integration.tar.zst https://juno-snapshots.nethermind.io/files/sepolia-integration/latest
```

</TabItem>
</Tabs>

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
zstd -dc juno_mainnet.tar.zst | tar -xf - -b 2048 -C $HOME/snapshots/mainnet
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet (Pruned)">

```bash
mkdir $HOME/snapshots/mainnet-pruned/
```

```bash
# Extract to your snapshots directory
zstd -dc juno_mainnet_pruned.tar.zst | tar -xf - -b 2048 -C $HOME/snapshots/mainnet-pruned
```

</TabItem>
<TabItem value="sepolia" label="Sepolia">

```bash
mkdir $HOME/snapshots/sepolia/
```

```bash
# Extract to your snapshots directory
zstd -dc juno_sepolia.tar.zst | tar -xf - -b 2048 -C $HOME/snapshots/sepolia
```

</TabItem>
<TabItem value="sepolia-pruned" label="Sepolia (Pruned)">

```bash
mkdir $HOME/snapshots/sepolia-pruned/
```

```bash
# Extract to your snapshots directory
zstd -dc juno_sepolia_pruned.tar.zst | tar -xf - -b 2048 -C $HOME/snapshots/sepolia-pruned
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
mkdir $HOME/snapshots/sepolia-integration/
```

```bash
# Extract to your snapshots directory
zstd -dc juno_sepolia_integration.tar.zst | tar -xf - -b 2048 -C $HOME/snapshots/sepolia-integration
```

</TabItem>
</Tabs>

:::tip Extraction performance
Decompressing a `.tar.zst` archive is single-threaded by design, so low CPU usage during extraction is normal. To speed it up, keep the archive and the target directory on different disks when possible. Alternatively, `bsdtar -xf juno_mainnet.tar.zst -C $HOME/snapshots/mainnet` (package `libarchive-tools` on Ubuntu/Debian) decompresses and extracts in a single process with large buffers and is often faster. Note that the `tar` bundled with macOS may lack zstd support.
:::

</TabItem>
<TabItem value="streaming" label="Streaming">

Streaming downloads and extracts the snapshot in a single step, reducing required disk space to just the size of the extracted database, contrary to **Download and then extract**, which additionally needs space for the compressed archive.


##### 1. Install streaming dependencies

The commands below use `lftp`, which allows the `zstd` and `tar` context to survive a network error, restarting the transfer seamlessly and keeping the stream going even in the worst of network connections.

`pv` is optional but recommended, since it makes it possible to show a progress bar while streaming.

<Tabs groupId="streaming-tools">
<TabItem value="lftp-pv" label="lftp + pv" default>

<Tabs groupId="os">
<TabItem value="ubuntu" label="Ubuntu/Debian" default>

```bash
sudo apt-get install lftp pv
```

</TabItem>
<TabItem value="macos" label="macOS">

```bash
brew install lftp pv
```

</TabItem>
<TabItem value="rhel" label="RHEL/CentOS/Fedora">

```bash
sudo dnf install lftp pv
```

</TabItem>
</Tabs>

---

</TabItem>
<TabItem value="lftp" label="lftp">

<Tabs groupId="os">
<TabItem value="ubuntu" label="Ubuntu/Debian" default>

```bash
sudo apt-get install lftp
```

</TabItem>
<TabItem value="macos" label="macOS">

```bash
brew install lftp
```

</TabItem>
<TabItem value="rhel" label="RHEL/CentOS/Fedora">

```bash
sudo dnf install lftp
```

</TabItem>
</Tabs>

---

</TabItem>
<TabItem value="curl" label="curl">

Nothing to install — `curl` is preinstalled on most systems.

---

</TabItem>
</Tabs>

##### 2. Stream the snapshot

Create a subfolder inside `$HOME/snapshots` where to stream the download, then download and extract the snapshot directly to your target directory.

With the `lftp + pv` variant, the command first asks the server for the archive size (`JUNO_SNAPSHOT_SIZE`), so `pv` can render a full progress bar with percentage and ETA alongside the bytes downloaded and the current transfer rate; if the lookup fails, `pv` falls back to a plain byte counter. Note that a rate that briefly drops to `0 B/s` usually just means lftp is reconnecting after a network error; reach out to the team if you're having issues with snapshot streaming.

<Tabs groupId="streaming-tools">
<TabItem value="lftp-pv" label="lftp + pv" default>

<Tabs groupId="network">
<TabItem value="mainnet" label="Mainnet" default>

```bash
mkdir -p $HOME/snapshots/mainnet

JUNO_SNAPSHOT_URL=https://juno-snapshots.nethermind.io/files/mainnet/latest
JUNO_SNAPSHOT_SIZE=$(curl -sIL "$JUNO_SNAPSHOT_URL" | tr -d '\r' | awk 'tolower($1)=="content-length:"{s=$2} END{print s}')
lftp -c "cat $JUNO_SNAPSHOT_URL" \
  | pv ${JUNO_SNAPSHOT_SIZE:+-s "$JUNO_SNAPSHOT_SIZE"} \
  | zstd -d | tar -xf - -C $HOME/snapshots/mainnet
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet (Pruned)">

```bash
mkdir -p $HOME/snapshots/mainnet-pruned

JUNO_SNAPSHOT_URL=https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest
JUNO_SNAPSHOT_SIZE=$(curl -sIL "$JUNO_SNAPSHOT_URL" | tr -d '\r' | awk 'tolower($1)=="content-length:"{s=$2} END{print s}')
lftp -c "cat $JUNO_SNAPSHOT_URL" \
  | pv ${JUNO_SNAPSHOT_SIZE:+-s "$JUNO_SNAPSHOT_SIZE"} \
  | zstd -d | tar -xf - -C $HOME/snapshots/mainnet-pruned
```

</TabItem>
<TabItem value="sepolia" label="Sepolia">

```bash
mkdir -p $HOME/snapshots/sepolia

JUNO_SNAPSHOT_URL=https://juno-snapshots.nethermind.io/files/sepolia/latest
JUNO_SNAPSHOT_SIZE=$(curl -sIL "$JUNO_SNAPSHOT_URL" | tr -d '\r' | awk 'tolower($1)=="content-length:"{s=$2} END{print s}')
lftp -c "cat $JUNO_SNAPSHOT_URL" \
  | pv ${JUNO_SNAPSHOT_SIZE:+-s "$JUNO_SNAPSHOT_SIZE"} \
  | zstd -d | tar -xf - -C $HOME/snapshots/sepolia
```

</TabItem>
<TabItem value="sepolia-pruned" label="Sepolia (Pruned)">

```bash
mkdir -p $HOME/snapshots/sepolia-pruned

JUNO_SNAPSHOT_URL=https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest
JUNO_SNAPSHOT_SIZE=$(curl -sIL "$JUNO_SNAPSHOT_URL" | tr -d '\r' | awk 'tolower($1)=="content-length:"{s=$2} END{print s}')
lftp -c "cat $JUNO_SNAPSHOT_URL" \
  | pv ${JUNO_SNAPSHOT_SIZE:+-s "$JUNO_SNAPSHOT_SIZE"} \
  | zstd -d | tar -xf - -C $HOME/snapshots/sepolia-pruned
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
mkdir -p $HOME/snapshots/sepolia-integration

JUNO_SNAPSHOT_URL=https://juno-snapshots.nethermind.io/files/sepolia-integration/latest
JUNO_SNAPSHOT_SIZE=$(curl -sIL "$JUNO_SNAPSHOT_URL" | tr -d '\r' | awk 'tolower($1)=="content-length:"{s=$2} END{print s}')
lftp -c "cat $JUNO_SNAPSHOT_URL" \
  | pv ${JUNO_SNAPSHOT_SIZE:+-s "$JUNO_SNAPSHOT_SIZE"} \
  | zstd -d | tar -xf - -C $HOME/snapshots/sepolia-integration
```

</TabItem>
</Tabs>

</TabItem>
<TabItem value="lftp" label="lftp">

<Tabs groupId="network">
<TabItem value="mainnet" label="Mainnet" default>

```bash
mkdir -p $HOME/snapshots/mainnet

lftp -c "cat https://juno-snapshots.nethermind.io/files/mainnet/latest" \
  | zstd -d | tar -xf - -C $HOME/snapshots/mainnet
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet (Pruned)">

```bash
mkdir -p $HOME/snapshots/mainnet-pruned

lftp -c "cat https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest" \
  | zstd -d | tar -xf - -C $HOME/snapshots/mainnet-pruned
```

</TabItem>
<TabItem value="sepolia" label="Sepolia">

```bash
mkdir -p $HOME/snapshots/sepolia

lftp -c "cat https://juno-snapshots.nethermind.io/files/sepolia/latest" \
  | zstd -d | tar -xf - -C $HOME/snapshots/sepolia
```

</TabItem>
<TabItem value="sepolia-pruned" label="Sepolia (Pruned)">

```bash
mkdir -p $HOME/snapshots/sepolia-pruned

lftp -c "cat https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest" \
  | zstd -d | tar -xf - -C $HOME/snapshots/sepolia-pruned
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
mkdir -p $HOME/snapshots/sepolia-integration

lftp -c "cat https://juno-snapshots.nethermind.io/files/sepolia-integration/latest" \
  | zstd -d | tar -xf - -C $HOME/snapshots/sepolia-integration
```

</TabItem>
</Tabs>

</TabItem>
<TabItem value="curl" label="curl">


<Tabs groupId="network">
<TabItem value="mainnet" label="Mainnet" default>

```bash
mkdir -p $HOME/snapshots/mainnet

curl -s -L https://juno-snapshots.nethermind.io/files/mainnet/latest \
| zstd -d | tar -xf - -C $HOME/snapshots/mainnet
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet (Pruned)">

```bash
mkdir -p $HOME/snapshots/mainnet-pruned

curl -s -L https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest \
| zstd -d | tar -xf - -C $HOME/snapshots/mainnet-pruned
```

</TabItem>
<TabItem value="sepolia" label="Sepolia">

```bash
mkdir -p $HOME/snapshots/sepolia

curl -s -L https://juno-snapshots.nethermind.io/files/sepolia/latest \
| zstd -d | tar -xf - -C $HOME/snapshots/sepolia
```

</TabItem>
<TabItem value="sepolia-pruned" label="Sepolia (Pruned)">

```bash
mkdir -p $HOME/snapshots/sepolia-pruned

curl -s -L https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest \
| zstd -d | tar -xf - -C $HOME/snapshots/sepolia-pruned
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
mkdir -p $HOME/snapshots/sepolia-integration

curl -s -L https://juno-snapshots.nethermind.io/files/sepolia-integration/latest \
| zstd -d | tar -xf - -C $HOME/snapshots/sepolia-integration
```

</TabItem>
</Tabs>


:::warning
Streaming with `curl` is unreliable: any network interruption forces a restart from scratch. Use it only if you cannot install lftp.
:::

</TabItem>
</Tabs>


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
  -p 6061:6061 \
  -v $HOME/snapshots/mainnet:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --ws \
  --ws-port 6061 \
  --ws-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --eth-node <YOUR-ETH-NODE>
```

</TabItem>
<TabItem value="mainnet-pruned" label="Mainnet (Pruned)">

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -p 6061:6061 \
  -v $HOME/snapshots/mainnet-pruned:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --ws \
  --ws-port 6061 \
  --ws-host 0.0.0.0 \
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
  -p 6061:6061 \
  -v $HOME/snapshots/sepolia:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --ws \
  --ws-port 6061 \
  --ws-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --network sepolia \
  --eth-node <YOUR-ETH-NODE>
```

</TabItem>
<TabItem value="sepolia-pruned" label="Sepolia (Pruned)">

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -p 6061:6061 \
  -v $HOME/snapshots/sepolia-pruned:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --ws \
  --ws-port 6061 \
  --ws-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --network sepolia \
  --prune-mode \
  --eth-node <YOUR-ETH-NODE>
```

</TabItem>
<TabItem value="sepolia-integration" label="Sepolia-Integration">

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -p 6061:6061 \
  -v $HOME/snapshots/sepolia-integration:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --ws \
  --ws-port 6061 \
  --ws-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --network sepolia-integration \
  --eth-node <YOUR-ETH-NODE>
```

</TabItem>
</Tabs>

:::info
Replace `<YOUR-ETH-NODE>` with your Ethereum node WebSocket URL, and make sure it matches the network's L1: Starknet Mainnet settles on Ethereum Mainnet (e.g. `wss://mainnet.infura.io/ws/v3/your-project-id`), while Sepolia and Sepolia-Integration settle on Ethereum Sepolia (e.g. `wss://sepolia.infura.io/ws/v3/your-project-id`). Ensure you use the WebSocket URL (`ws`/`wss`) instead of the HTTP URL (`http`/`https`).
:::

:::tip
These examples use Docker. For other ways to run Juno (standalone binary, building from source) and more configuration details, see [Running Juno](running-juno) guide.
:::

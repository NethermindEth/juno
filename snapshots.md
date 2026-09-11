# Sync from a Snapshot

It is possible to avoid syncing from the beginning and waiting weeks to catch up by downloading a Juno snapshot. You're downloading a pre-synced Juno database that you can point your node to. This will reduce the syncing time to just a few hours.

Snapshots are provided in a compressed `.tar.zst` format for faster downloads and reduced storage requirements. It also allows you to directly stream the decompressed file to your computer without needing to download it first.

Additionally, _pruned_ snapshots are offered. They contain only the latest data, greatly reducing storage size.

## Network Snapshots

| Network             | Download Link                                                                                                 |
| ------------------- | ------------------------------------------------------------------------------------------------------------- |
| Mainnet             | [**juno_mainnet.tar.zst**](https://juno-snapshots.nethermind.io/files/mainnet/latest)                         |
| Mainnet (Pruned)    | [**juno_mainnet_pruned.tar.zst**](https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest)           |
| Sepolia             | [**juno_sepolia.tar.zst**](https://juno-snapshots.nethermind.io/files/sepolia/latest)                         |
| Sepolia (Pruned)    | [**juno_sepolia_pruned.tar.zst**](https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest)           |
| Sepolia-Integration | [**juno_sepolia_integration.tar.zst**](https://juno-snapshots.nethermind.io/files/sepolia-integration/latest) |

:::tip
Select your network in any tab below and the rest of the page follows. The choice is synced across every command on this page.
:::

## Getting snapshot sizes

Snapshot sizes as of `Fri Jul 31 2026`:

### Mainnet

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/mainnet/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 453.93 GB
```

### Mainnet (Pruned)

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 85.63 GB
```

### Sepolia

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 77.14 GB
```

### Sepolia (Pruned)

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 23.56 GB
```

### Sepolia-Integration

```bash
curl -s -I -L https://juno-snapshots.nethermind.io/files/sepolia-integration/latest | gawk -v IGNORECASE=1 '/^Content-Length/ { printf "%.2f GB\n", $2/1024/1024/1024 }'
# 38.64 GB
```

## Run Juno with a snapshot

You can either stream the snapshot directly into the target directory without storing the archive, or download the snapshot archive and then extract it. All commands below run in your current directory, so `cd` into the location where you want the snapshot first. Both methods share the first step:

### 1. Install zstd

[zstd (Zstandard)](https://github.com/facebook/zstd) is required to decompress and directly stream the snapshots into your system without requiring temporary storage. `zstd` provides significantly better compression ratios and faster decompression speeds compared to traditional tar compression.

### Ubuntu/Debian

```bash
sudo apt-get install zstd
```

### macOS

```bash
brew install zstd
```

### RHEL/CentOS/Fedora

```bash
sudo dnf install zstd
```

### 2. Get the snapshot

### Stream

Streaming downloads and extracts the snapshot in a single step, reducing required disk space to just the size of the extracted database, contrary to **Download**, which additionally needs space for the compressed archive.

##### 1. Install streaming dependencies

Streaming requires either `wget` or `lftp` installed on your computer.

These allow the `zstd` and `tar` context to survive a network error, restarting the transfer seamlessly and keeping the stream going even in the worst of network connections.

### wget

`wget` is usually preinstalled on Linux distributions. If missing:

### Ubuntu/Debian

```bash
sudo apt-get install wget
```

### macOS

```bash
brew install wget
```

### RHEL/CentOS/Fedora

```bash
sudo dnf install wget
```

---

### lftp + pv

`pv` is an optional dependency for showing a progress bar while executing the `lftp` command.

### Ubuntu/Debian

```bash
sudo apt-get install lftp pv
```

### macOS

```bash
brew install lftp pv
```

### RHEL/CentOS/Fedora

```bash
sudo dnf install lftp pv
```

---

### curl

Nothing to install. `curl` is preinstalled on most systems.

---

##### 2. Stream the snapshot

Create a subfolder in your current directory where to stream the download, then download and extract the snapshot directly to your target directory:

### Mainnet

```bash
mkdir -p juno_mainnet
```

### Mainnet (Pruned)

```bash
mkdir -p juno_mainnet_pruned
```

### Sepolia

```bash
mkdir -p juno_sepolia
```

### Sepolia (Pruned)

```bash
mkdir -p juno_sepolia_pruned
```

### Sepolia-Integration

```bash
mkdir -p juno_sepolia_integration
```

Stream the data to your computer: 

1. `wget` streams the data reliably and comes preinstalled on most systems.
2. `lftp + pv` is a solid alternative if you've no access to `wget`.
3. `curl` gives no guarantees. Use it only as a last resort.

### wget

### Mainnet

```bash
wget --tries=0 --retry-connrefused --retry-on-http-error=500,502,503,504 --read-timeout=60 -O - \
  https://juno-snapshots.nethermind.io/files/mainnet/latest \
| zstd -d | tar -xf - -C juno_mainnet
```

### Mainnet (Pruned)

```bash
wget --tries=0 --retry-connrefused --retry-on-http-error=500,502,503,504 --read-timeout=60 -O - \
  https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest \
| zstd -d | tar -xf - -C juno_mainnet_pruned
```

### Sepolia

```bash
wget --tries=0 --retry-connrefused --retry-on-http-error=500,502,503,504 --read-timeout=60 -O - \
  https://juno-snapshots.nethermind.io/files/sepolia/latest \
| zstd -d | tar -xf - -C juno_sepolia
```

### Sepolia (Pruned)

```bash
wget --tries=0 --retry-connrefused --retry-on-http-error=500,502,503,504 --read-timeout=60 -O - \
  https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest \
| zstd -d | tar -xf - -C juno_sepolia_pruned
```

### Sepolia-Integration

```bash
wget --tries=0 --retry-connrefused --retry-on-http-error=500,502,503,504 --read-timeout=60 -O - \
  https://juno-snapshots.nethermind.io/files/sepolia-integration/latest \
| zstd -d | tar -xf - -C juno_sepolia_integration
```

### lftp + pv

### Mainnet

```bash
JUNO_SNAPSHOT_URL=https://juno-snapshots.nethermind.io/files/mainnet/latest
JUNO_SNAPSHOT_SIZE=$(curl -sIL "$JUNO_SNAPSHOT_URL" | tr -d '\r' | awk 'tolower($1)=="content-length:"{s=$2} END{print s}')
lftp -c "cat $JUNO_SNAPSHOT_URL" \
  | pv ${JUNO_SNAPSHOT_SIZE:+-s "$JUNO_SNAPSHOT_SIZE"} \
  | zstd -d | tar -xf - -C juno_mainnet
```

### Mainnet (Pruned)

```bash
JUNO_SNAPSHOT_URL=https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest
JUNO_SNAPSHOT_SIZE=$(curl -sIL "$JUNO_SNAPSHOT_URL" | tr -d '\r' | awk 'tolower($1)=="content-length:"{s=$2} END{print s}')
lftp -c "cat $JUNO_SNAPSHOT_URL" \
  | pv ${JUNO_SNAPSHOT_SIZE:+-s "$JUNO_SNAPSHOT_SIZE"} \
  | zstd -d | tar -xf - -C juno_mainnet_pruned
```

### Sepolia

```bash
JUNO_SNAPSHOT_URL=https://juno-snapshots.nethermind.io/files/sepolia/latest
JUNO_SNAPSHOT_SIZE=$(curl -sIL "$JUNO_SNAPSHOT_URL" | tr -d '\r' | awk 'tolower($1)=="content-length:"{s=$2} END{print s}')
lftp -c "cat $JUNO_SNAPSHOT_URL" \
  | pv ${JUNO_SNAPSHOT_SIZE:+-s "$JUNO_SNAPSHOT_SIZE"} \
  | zstd -d | tar -xf - -C juno_sepolia
```

### Sepolia (Pruned)

```bash
JUNO_SNAPSHOT_URL=https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest
JUNO_SNAPSHOT_SIZE=$(curl -sIL "$JUNO_SNAPSHOT_URL" | tr -d '\r' | awk 'tolower($1)=="content-length:"{s=$2} END{print s}')
lftp -c "cat $JUNO_SNAPSHOT_URL" \
  | pv ${JUNO_SNAPSHOT_SIZE:+-s "$JUNO_SNAPSHOT_SIZE"} \
  | zstd -d | tar -xf - -C juno_sepolia_pruned
```

### Sepolia-Integration

```bash
JUNO_SNAPSHOT_URL=https://juno-snapshots.nethermind.io/files/sepolia-integration/latest
JUNO_SNAPSHOT_SIZE=$(curl -sIL "$JUNO_SNAPSHOT_URL" | tr -d '\r' | awk 'tolower($1)=="content-length:"{s=$2} END{print s}')
lftp -c "cat $JUNO_SNAPSHOT_URL" \
  | pv ${JUNO_SNAPSHOT_SIZE:+-s "$JUNO_SNAPSHOT_SIZE"} \
  | zstd -d | tar -xf - -C juno_sepolia_integration
```

### curl

### Mainnet

```bash
curl -s -L https://juno-snapshots.nethermind.io/files/mainnet/latest \
| zstd -d | tar -xf - -C juno_mainnet
```

### Mainnet (Pruned)

```bash
curl -s -L https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest \
| zstd -d | tar -xf - -C juno_mainnet_pruned
```

### Sepolia

```bash
curl -s -L https://juno-snapshots.nethermind.io/files/sepolia/latest \
| zstd -d | tar -xf - -C juno_sepolia
```

### Sepolia (Pruned)

```bash
curl -s -L https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest \
| zstd -d | tar -xf - -C juno_sepolia_pruned
```

### Sepolia-Integration

```bash
curl -s -L https://juno-snapshots.nethermind.io/files/sepolia-integration/latest \
| zstd -d | tar -xf - -C juno_sepolia_integration
```

:::warning
Streaming with `curl` is unreliable: any network interruption forces a restart from scratch. Use it only if you cannot use wget or lftp.
:::

### Download

Two-step approach where we first download the snapshot and extract it later. Note that this will create the requirement to have twice the space required for the Juno snapshot. If space is not enough, use the **Stream** tab instead.

##### 1. Download the snapshot

Both `wget --continue` and `curl -C -` resume an interrupted download: if the transfer dies for any reason, re-run the same command and it continues from where it stopped.

### Mainnet

### wget

```bash
wget --continue -O "juno_mainnet.tar.zst" https://juno-snapshots.nethermind.io/files/mainnet/latest
```

### curl

```bash
curl -L -C - -o juno_mainnet.tar.zst https://juno-snapshots.nethermind.io/files/mainnet/latest
```

### Mainnet (Pruned)

### wget

```bash
wget --continue -O "juno_mainnet_pruned.tar.zst" https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest
```

### curl

```bash
curl -L -C - -o juno_mainnet_pruned.tar.zst https://juno-snapshots.nethermind.io/files/mainnet-pruned/latest
```

### Sepolia

### wget

```bash
wget --continue -O "juno_sepolia.tar.zst" https://juno-snapshots.nethermind.io/files/sepolia/latest
```

### curl

```bash
curl -L -C - -o juno_sepolia.tar.zst https://juno-snapshots.nethermind.io/files/sepolia/latest
```

### Sepolia (Pruned)

### wget

```bash
wget --continue -O "juno_sepolia_pruned.tar.zst" https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest
```

### curl

```bash
curl -L -C - -o juno_sepolia_pruned.tar.zst https://juno-snapshots.nethermind.io/files/sepolia-pruned/latest
```

### Sepolia-Integration

### wget

```bash
wget --continue -O "juno_sepolia_integration.tar.zst" https://juno-snapshots.nethermind.io/files/sepolia-integration/latest
```

### curl

```bash
curl -L -C - -o juno_sepolia_integration.tar.zst https://juno-snapshots.nethermind.io/files/sepolia-integration/latest
```

##### 2. Extract the snapshot

Create a subfolder in your current directory where to unzip the downloaded snapshot:

### Mainnet

```bash
mkdir juno_mainnet
```

```bash
# Extract the snapshot
zstd -dc juno_mainnet.tar.zst | tar -xf - -b 2048 -C juno_mainnet
```

### Mainnet (Pruned)

```bash
mkdir juno_mainnet_pruned
```

```bash
# Extract the snapshot
zstd -dc juno_mainnet_pruned.tar.zst | tar -xf - -b 2048 -C juno_mainnet_pruned
```

### Sepolia

```bash
mkdir juno_sepolia
```

```bash
# Extract the snapshot
zstd -dc juno_sepolia.tar.zst | tar -xf - -b 2048 -C juno_sepolia
```

### Sepolia (Pruned)

```bash
mkdir juno_sepolia_pruned
```

```bash
# Extract the snapshot
zstd -dc juno_sepolia_pruned.tar.zst | tar -xf - -b 2048 -C juno_sepolia_pruned
```

### Sepolia-Integration

```bash
mkdir juno_sepolia_integration
```

```bash
# Extract the snapshot
zstd -dc juno_sepolia_integration.tar.zst | tar -xf - -b 2048 -C juno_sepolia_integration
```

## Running Juno with snapshots

### 1. Run Juno

From the same directory where you extracted or streamed the snapshot, run the Docker command to start Juno:

### Mainnet

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -p 6061:6061 \
  -v $(pwd)/juno_mainnet:/var/lib/juno \
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

### Mainnet (Pruned)

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -p 6061:6061 \
  -v $(pwd)/juno_mainnet_pruned:/var/lib/juno \
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

### Sepolia

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -p 6061:6061 \
  -v $(pwd)/juno_sepolia:/var/lib/juno \
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

### Sepolia (Pruned)

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -p 6061:6061 \
  -v $(pwd)/juno_sepolia_pruned:/var/lib/juno \
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

### Sepolia-Integration

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -p 6061:6061 \
  -v $(pwd)/juno_sepolia_integration:/var/lib/juno \
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

:::info
Replace `<YOUR-ETH-NODE>` with your Ethereum node WebSocket URL, and make sure it matches the network's L1: Starknet Mainnet settles on Ethereum Mainnet (e.g. `wss://mainnet.infura.io/ws/v3/your-project-id`), while Sepolia and Sepolia-Integration settle on Ethereum Sepolia (e.g. `wss://sepolia.infura.io/ws/v3/your-project-id`). Ensure you use the WebSocket URL (`ws`/`wss`) instead of the HTTP URL (`http`/`https`).
:::

:::tip
These examples use Docker. For other ways to run Juno (standalone binary, building from source) and more configuration details, see the [Installation](running-juno) guide.
:::

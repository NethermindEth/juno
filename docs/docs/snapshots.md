---
title: Database Snapshots
---

# Database Snapshots :camera_flash:

You can download a snapshot of the Juno database to shorten the network syncing time. Only the blocks added after the snapshot will be synced when running the Juno node.

## Mainnet

| Version      | Size       | Block      | Download Link                                                                                         |
| ------------ | ---------- | ---------- | ----------------------------------------------------------------------------------------------------- |
| **>=v0.9.2** | **182 GB** | **640855** | [**juno_mainnet.tar**](https://juno-snapshots.nethermind.dev/mainnet/juno_mainnet_v0.11.7_640855.tar) |

## Sepolia

| Version      | Size     | Block     | Download Link                                                                                        |
| ------------ | -------- | --------- | ---------------------------------------------------------------------------------------------------- |
| **>=v0.9.2** | **5 GB** | **66477** | [**juno_sepolia.tar**](https://juno-snapshots.nethermind.dev/sepolia/juno_sepolia_v0.11.7_66477.tar) |

## Run Juno with a snapshot

### 1. Download the snapshot

First, download a snapshot from one of the provided URLs:

```bash
wget -O juno_mainnet.tar https://juno-snapshots.nethermind.dev/mainnet/juno_mainnet_v0.11.7_640855.tar
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

Run the Docker command to start Juno, ensuring to specify the correct path to the snapshot:

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/snapshots/juno_mainnet:/var/lib/juno \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --eth-node <YOUR ETH NODE>
```

:::info
Replace \<YOUR ETH NODE\> with the WebSocket endpoint of your Ethereum node. For Infura users, your address should be: `wss://mainnet.infura.io/ws/v3/your-infura-project-id`. Ensure you use the WebSocket URL (`ws`/`wss`) instead of the HTTP URL (`http`/`https`).
:::

After completing these steps, Juno should be up and running on your system using the snapshot.

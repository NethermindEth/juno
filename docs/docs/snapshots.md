---
title: Database Snapshots
---

You can download a snapshot of the Juno database to shorten the network syncing time. Only the blocks added after the snapshot will be synced when running the Juno node.

## Mainnet

| Version      | Size       | Block      | Download Link                                                                                        |
| ------------ | ---------- | ---------- | ---------------------------------------------------------------------------------------------------- |
| **>=v0.9.2** | **156 GB** | **519634** | [**juno_mainnet.tar**](https://juno-snapshots.nethermind.dev/mainnet/juno_mainnet_v0.9.3_519634.tar) |

## Sepolia

| Version      | Size       | Block     | Download Link                                                                                        |
| ------------ | ---------- | --------- | ---------------------------------------------------------------------------------------------------- |
| **>=v0.9.2** | **2.9 GB** | **55984** | [**juno_sepolia.tar**](https://juno-snapshots.nethermind.dev/sepolia/juno_sepolia_v0.11.4_55984.tar) |

## Run Juno with a snapshot

#### 1. Download the snapshot

First, download a snapshot from one of the provided URLs:

```bash
curl -o juno_mainnet_519634.tar https://juno-snapshots.nethermind.dev/mainnet/juno_mainnet_v0.9.3_519634.tar
```

#### 2. Prepare a directory

Ensure you have a directory to store the snapshots. We will use the `$HOME/snapshots` directory:

```bash
mkdir -p $HOME/snapshots
```

#### 3. Extract the snapshot

Extract the contents of the downloaded `.tar` file into the directory:

```bash
tar -xvf juno_mainnet_519634.tar -C $HOME/snapshots
```

#### 4. Run Juno

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
Replace \<YOUR ETH NODE\> with the WebSocket endpoint of your Ethereum node.
:::

After completing these steps, Juno should be up and running on your system using the snapshot.

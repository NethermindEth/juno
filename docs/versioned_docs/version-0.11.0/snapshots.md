---
slug: /snapshots
sidebar_position: 4
title: Database Snapshots
---

To decrease sync times, users may opt to download a Juno database snapshot.
After downloading a snapshot and starting a Juno node, only recent blocks must be synced.

## Mainnet

| Version | Size | Block | Download Link |
| ------- | ---- | ----- | ------------- |
| **>=v0.9.2**  | **182 GB** | **640855** | [**juno_mainnet.tar**](https://juno-snapshots.nethermind.io/mainnet/juno_mainnet_v0.11.7_640855.tar) |


## Sepolia

| Version | Size | Block | Download Link |
| ------- | ---- | ----- | ------------- |
| **>=v0.9.2** | **5 GB** | **66477** | [**juno_sepolia.tar**](https://juno-snapshots.nethermind.io/sepolia/juno_sepolia_v0.11.7_66477.tar) |

## Run Juno Using Snapshot

1. **Download Snapshot**

   Fetch a snapshot from one of the provided URLs:

   ```bash
   wget -O juno_mainnet.tar https://juno-snapshots.nethermind.io/mainnet/juno_mainnet_v0.11.7_640855.tar
   ```

2. **Prepare Directory**

   Ensure you have a directory where you will store the snapshots. We will use `$HOME/snapshots`.

   ```bash
   mkdir -p $HOME/snapshots
   ```

3. **Extract Tarball**

   Extract the contents of the `.tar` file:

   ```bash
   tar -xvf juno_mainnet.tar -C $HOME/snapshots
   ```

4. **Run Juno**

   Execute the Docker command to run Juno, ensuring that you're using the correct snapshot path `$HOME/snapshots/juno_mainnet`:

   ```bash
   docker run -d \
      --name juno \
      -p 6060:6060 \
      -v $HOME/snapshots/juno_mainnet:/var/lib/juno \
      nethermind/juno \
      --http \
      --http-port 6060 \
      --http-host 0.0.0.0 \
      --db-path /var/lib/juno
   ```

After following these steps, Juno should be up and running on your machine, utilizing the provided snapshot.

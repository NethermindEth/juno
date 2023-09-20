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
| **>=v0.6.0**  | **49.2 GB** | **166353.tar** | [**juno_mainnet_166353.tar**](https://pub-932514831f0f4245850f7a471132e564.r2.dev/mainnet/juno_mainnet_v0.6.0_166353.tar) |

## Goerli

| Version | Size | Block | Download Link |
| ------- | ---- | ----- | ------------- |
| **>=v0.6.0** | **36 GB** | **850192** | [**juno_goerli_850192.tar**](https://pub-932514831f0f4245850f7a471132e564.r2.dev/goerli/juno_goerli_v0.6.0_850192.tar) |

## Goerli2

| Version | Size | Block | Download Link |
| ------- | ---- | ----- | ------------- |
| **>=v0.6.0** | **4.6 GB** | **139043** | [**juno_goerli2_135973.tar**](https://pub-932514831f0f4245850f7a471132e564.r2.dev/goerli2/juno_goerli2_v0.6.0_139043.tar) |

## Run Juno Using Snapshot

1. **Download Snapshot**

   Fetch a snapshot from one of the provided URLs:

   ```bash
   curl -o juno_mainnet_166353.tar https://pub-932514831f0f4245850f7a471132e564.r2.dev/mainnet/juno_mainnet_v0.6.0_166353.tar
   ```

2. **Prepare Directory**

   Ensure you have a directory where you will store the snapshots. We will use `$HOME/snapshots`.

   ```bash
   mkdir -p $HOME/snapshots
   ```

3. **Extract Tarball**

   Extract the contents of the `.tar` file:

   ```bash
   tar -xvf juno_mainnet_166353 -C $HOME/snapshots
   ```

4. **Run Juno**

   Execute the Docker command to run Juno, ensuring that you're using the correct snapshot path `$HOME/snapshots/juno_mainnet`:

   ```bash
   docker run -d \
     --name juno \
     -p 6060:6060 \
     -v $HOME/snapshots/juno_mainnet:/var/lib/juno \
     nethermind/juno \
     --http-port 6060 \
     --db-path /var/lib/juno
   ```

After following these steps, Juno should be up and running on your machine, utilizing the provided snapshot.

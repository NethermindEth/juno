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
| >=v0.4.0  | 29.4 GB | 100713 | [juno_mainnet_100713.tar](https://juno-snapshot.s3.us-east-2.amazonaws.com/mainnet/juno_mainnet_v0.4.0_100713.tar) |
| **>=v0.4.0**  | **36.5 GB** | **136902** | [**juno_mainnet_136902.tar**](https://juno-snapshot.s3.us-east-2.amazonaws.com/mainnet/juno_mainnet_v0.5.0_136902.tar) |

## Goerli

| Version | Size | Block | Download Link |
| ------- | ---- | ----- | ------------- |
| >=v0.4.0 | 31.6 GB | 830385 | [juno_goerli_830385.tar](https://juno-snapshot.s3.us-east-2.amazonaws.com/goerli/juno_goerli_v0.4.0_830385.tar) |
| **>=v0.4.0** | **32.3 GB** | **839969** | [**juno_goerli_839969.tar**](https://juno-snapshot.s3.us-east-2.amazonaws.com/goerli/juno_goerli_v0.5.0_839969.tar) |

## Goerli2

| Version | Size | Block | Download Link |
| ------- | ---- | ----- | ------------- |
| >=v0.4.0 | 1.8 GB | 125026 | [juno_goerli2_125026.tar](https://juno-snapshot.s3.us-east-2.amazonaws.com/goerli2/juno_goerli2_v0.4.0_125026.tar) |
| **>=v0.4.0** | **4.5 GB** | **135973** | [**juno_goerli2_135973.tar**](https://juno-snapshot.s3.us-east-2.amazonaws.com/goerli2/juno_goerli2_v0.5.0_135973.tar) |

## Run Juno Using Snapshot

1. **Download Snapshot**

   Fetch a snapshot from one of the provided URLs:

   ```bash
   curl -o juno_mainnet_v0.5.0_136902.tar https://juno-snapshot.s3.us-east-2.amazonaws.com/mainnet/juno_mainnet_v0.5.0_136902.tar
   ```

2. **Prepare Directory**

   Ensure you have a directory where you will store the snapshots. We will use `$HOME/snapshots`.

   ```bash
   mkdir -p $HOME/snapshots
   ```

3. **Extract Tarball**

   Extract the contents of the `.tar` file:

   ```bash
   tar -xvf juno_mainnet_v0.5.0_136902.tar -C $HOME/snapshots
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

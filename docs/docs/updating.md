---
title: Updating Juno
---

# Updating Juno :arrows_counterclockwise:

```mdx-code-block
import Tabs from "@theme/Tabs";
import TabItem from "@theme/TabItem";
```

It is important to run the latest version of Juno as each update brings new features, security patches, and improvements over previous versions. Follow these steps to update Juno.

:::info
When running an updated node, use the same `db-path` as before to avoid restarting the sync and use the already synced database.
:::

<Tabs groupId="install">
<TabItem value="docker" label="Docker" default>

**1. Get the latest Docker image**

Download the latest Juno Docker image from the [nethermind/juno](https://hub.docker.com/r/nethermind/juno) repository:

```bash
docker pull nethermind/juno:latest
```

**2. Stop and remove the current Juno container**

Stop the currently running Juno container. If you're unsure of the container name, use `docker ps` to view all running containers:

```bash
docker stop juno
```

Remove the old container to prevent any conflicts with the new version:

```bash
docker rm juno
```

**3. Start a new container with the updated image**

Run a new container using the updated Docker image:

```bash
# Prepare the snapshots directory
mkdir -p $HOME/snapshots

# Run the container
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/snapshots/juno_mainnet:/snapshots/juno_mainnet \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --eth-node <YOUR-ETH-NODE> \
  --db-path /snapshots/juno_mainnet
```

Verify that the node is running correctly with the updated version:

```bash
docker logs juno
```

</TabItem>
<TabItem value="binary" label="Standalone Binary">

Download the latest binary from the [Juno GitHub Releases](https://github.com/NethermindEth/juno/releases/latest) page and replace the existing one.

</TabItem>
</Tabs>

## Updating from source

<Tabs groupId="install">
<TabItem value="docker" label="Docker" default>

Pull the latest changes and rebuild the Docker image:

```bash
# Pull the latest updates to the codebase
git pull

# Rebuild the Docker image
docker build -t nethermind/juno:latest .
```

Then stop, remove, and start the container again with the rebuilt image, as shown in the **Docker** tab above.

</TabItem>
<TabItem value="binary" label="Standalone Binary">

You can update the standalone binary by pulling the latest changes from GitHub and rebuilding it.


```bash
# Pull the latest updates to the codebase
git pull

# Rebuild the binary
make juno
```

See [Building from source](running-juno#building-from-source) for the full prerequisites and build steps.

</TabItem>
</Tabs>

:::tip
To learn how to configure Juno, check out the [Configuring Juno](configuring) guide.
:::

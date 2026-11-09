# Updating Juno

It is important to run the latest version of Juno as each update brings new features, security patches, and improvements over previous versions. Follow these steps to update Juno.

:::info
When running an updated node, use the same `db-path` as before to avoid restarting the sync and use the already synced database. The commands below use paths relative to your current directory, so run them from the same directory as the previous node.
:::

### Docker

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
# Prepare the database directory
mkdir -p juno_mainnet

# Run the container
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $(pwd)/juno_mainnet:/snapshots/juno_mainnet \
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

### Standalone Binary

1. Stop the node.
2. Run the download command for your platform from the [Installation](running-juno) page (**Standalone Binary** tab, step 1). It replaces `./juno` with the latest release.
3. Start `./juno` again with the same flags as before, including `--db-path`.

## Updating from source

### Docker

From your `juno` clone, fetch the release tags, switch to the latest release, and rebuild the Docker image:

```bash
# Switch to the latest release
git fetch --tags
TAG=$(git tag -l 'v*' --sort=-v:refname | grep -v -- - | head -n1)
git switch --detach "$TAG"

# Rebuild the Docker image
docker build -t nethermind/juno:latest .
```

Then stop, remove, and start the container again with the rebuilt image, as shown in the **Docker** tab above.
To build a version other than the latest, see [Building from source](running-juno#building-from-source).

### Standalone Binary

From your `juno` clone, fetch the release tags, switch to the latest release, and rebuild the binary:

```bash
# Switch to the latest release
git fetch --tags
TAG=$(git tag -l 'v*' --sort=-v:refname | grep -v -- - | head -n1)
git switch --detach "$TAG"

# Rebuild the binary
make juno
```

Then stop the node and start `./build/juno` again with the same `--db-path`.
See [Building from source](running-juno#building-from-source) for the prerequisites and for building a version other than the latest.

:::tip
To learn how to configure Juno, check out the [Configuration](configuring) guide.
:::

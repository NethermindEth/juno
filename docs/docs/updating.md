---
title: Updating Juno
---

# Updating Juno :arrows_counterclockwise:

It is important to run the latest version of Juno as each update brings new features, security patches, and improvements over previous versions. Follow these steps to update Juno:

- [Docker container](#docker-container)
- [Standalone binary](#standalone-binary)
- [Updating from source](#updating-from-source)

## Docker container

### 1. Get the latest Docker image

Download the latest Juno Docker image from the [nethermind/juno](https://hub.docker.com/r/nethermind/juno) repository:

```bash
docker pull nethermind/juno:latest
```

### 2. Stop and remove the current Juno container

Stop the currently running Juno container. If you're unsure of the container name, use `docker ps` to view all running containers:

```bash
docker stop juno
```

Remove the old container to prevent any conflicts with the new version:

```bash
docker rm juno
```

### 3. Start a new container with the updated image

Run a new container using the updated Docker image:

```bash
docker run -d \
  --name juno \
  -p 6060:6060 \
  nethermind/juno \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0
```

Verify that the node is running correctly with the updated version:

```bash
docker logs juno
```

## Standalone binary

Download the latest binary from [Juno's GitHub Releases](https://github.com/NethermindEth/juno/releases) page and replace the existing one.

## Updating from source

```bash
# Pull the latest updates to the codebase
git pull

# Rebuild the binary or Docker image
make juno

# OR

docker build -t nethermind/juno:latest .
```

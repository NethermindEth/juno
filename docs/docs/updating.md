---
title: Updating Juno
---

# Updating Juno :arrows_counterclockwise:

Updating your Juno node is crucial to access new features, improvements, and security patches. Follow these steps to update your node to the latest version using Docker.

### Steps to Update

1. **Pull the Latest Juno Docker Image**

First, pull the latest Juno Docker image from Nethermind's Docker repository. As an example, to update to `v0.11.0-rc1`, use the following command:

```
docker pull nethermind/juno:v0.11.0-rc1
```

2. **Stop the Current Juno Container**

Stop your currently running Juno container. If you're unsure of your container's name, you can use `docker ps` to list active containers.

```
docker stop juno
```

3. **Remove the Old Container**

Once the container is stopped, remove it to prevent any conflicts with the new version.

```
docker rm juno
```

4. **Start a New Container with the Updated Image**

Run a new container using the updated Docker image. Here's an example command, adjust it according to your setup (ports, volumes, version etc.):

```
docker run -d \
  --name juno \
  -p 6060:6060 \
  -v $HOME/juno:/var/lib/juno \
  nethermind/juno:v0.11.0-rc1 \
  --http \
  --http-port 6060 \
  --http-host 0.0.0.0 \
  --db-path /var/lib/juno \
  --eth-node <YOUR-ETH-NODE>
```

5. **Verify the Update**

After starting the new container, verify that the node is running correctly with the updated version.

```
docker logs juno
```

### Conclusion

You have successfully updated your Juno node to the latest version. It is now ready to be used. For more information on managing your node, visit [Nethermind's official GitHub repository](https://github.com/NethermindEth/juno).

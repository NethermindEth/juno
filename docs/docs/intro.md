---
title: Welcome
sidebar_position: 1
---

# Welcome to Juno

Let's discover **Juno in less than 5 minutes**.

## What You'll Need

- [Golang](https://go.dev/doc/install) version 1.18 for build and run the project.
- _For Linux_: You will need to install `clang`:

```shell
sudo apt -y install clang
```

### Installing

After cloning the project,

```bash
git clone https://github.com/NethermindEth/juno
```

You can install all the dependencies running the next command inside the project folder:

```bash
$ go get ./...
```

## Running Juno

### Compiling Directly

Compile Juno:

```bash
$ make all
```

After compilation, you will have two executables inside the `build` folder:

- juno
    - `juno` is the command that initializes the node.
- juno-cli
    - `juno-cli` is the command that direct interactions with the StarkNet ecosystem.

To synchronize with the StarkNet state from the centralized feeder gateway, run the following command:

```bash
$ ./build/juno --sync-enable
```

To sync the state without relying on the feeder gateway, configure an Ethereum node and run the following command:

```bash
$ ./build/juno --sync-enable --sync-ethnode <node-endpoint>
```

For more configuration details, check the [config description](https://gojuno.xyz/docs/running/config).

### Using Docker

If you prefer to use docker, you can follow [this](https://gojuno.xyz/docs/running/docker) guide.

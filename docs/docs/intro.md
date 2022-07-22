---
title: Quick Start
slug: /
hide_toc: false
sidebar_position: 1
pagination_next: null
---

This short guide shows you what Juno is and how to get started.

## What do you get with Juno?

- `juno`: a StarkNet node.
- `juno-cli`: commands to interact with the StarkNet network.

## Installation requirements

- [Golang](https://go.dev/doc/install) version 1.18 to build and run the project will usually be everything you need.

_Check [here](./download.mdx) for more information and OS specific instructions._

### Installing Juno from source

Clone the project and install all dependencies:

```bash
git clone https://github.com/NethermindEth/juno
cd juno
go get ./...
```
## Running Juno

_For more details on running the client, check [this page](./run/normal.mdx)._

To compile the tools (`juno` and `juno-cli`), simply run the `make compile` command within the project directory:

```bash
make compile
```

You will then have 2 available commands, generated inside the `build` folder of the project:

```bash
./build/juno # To start the node
./build/juno-cli # To explore the StarkNet network and its transactions
```

### Using Docker

To run the project inside a Docker container, read [here](./run/docker.mdx).

## Example usage

TODO: Would a user use make all to run Juno?

<fontcolor:red>
- How would a normal user want to use Juno? 
- Somebody who just wanted to interact with the network?
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

- [Golang](https://go.dev/doc/install) version 1.18.
- [Clang](https://clang.llvm.org/get_started.html) latest version. 

_Check [here](./download.mdx) for more information on getting Juno and OS specific instructions._

### Installing Juno from source

Clone the project and install all dependencies:

```bash
git clone https://github.com/NethermindEth/juno
cd juno
go get ./...
```
## Running Juno

_For more details on running the client, check [this page](./run/normal.mdx)._

To run Juno in your machine with the default settings, simply:


``` bash
make all
```

### Using Docker

To run the project inside a Docker container, read [here](./run/docker.mdx).

<!-- ## Example usage -->

TODO: Would a user use make all to run Juno?

- How would a normal user want to use Juno? 
- Somebody who just wanted to interact with the network?
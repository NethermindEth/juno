---
slug: welcome
title: Welcome
authors: [maceo]
tags: [hello, juno]
---

Juno is a Go implementation of a StarkNet full node client made with ❤️ by Nethermind.

We are working hard for our first release, until that happen, what you can do?

Let's discover **Juno in less than 5 min**.

## What you'll need

- [Golang](https://go.dev/doc/install) version 1.18 for build and run the project.
- [Cairo-lang](https://www.cairo-lang.org/docs/quickstart.html) if you want to do `starknet_call` command.

### Installing

You can get all the dependencies running the next command:

```bash
$ go get -u github.com/NethermindEth/juno 
```

## Running the node

### Compiling directly

If you want to run the node, you can compile directly the app using:

```bash
$ make compile
```

and after you execute the building process, you can run the node using this command:

```bash
$ make run
```

### Using Docker

If you want to keep your environment clean and run Juno in a Docker container, use the following command:

```bash 
docker run -p 8080:8080 -v juno_data:/home/app/.config/juno/data -v /path/to/your/config/file/juno.yaml:/home/app/.config/juno/juno.yaml juno
```

For more details, see the docker documentation page.

---
title: Welcome
sidebar_position: 1
---

# Welcome to Juno

Let's discover **Juno in less than 5 minutes**.

## What You'll Need

- [Golang](https://go.dev/doc/install) version 1.18 for build and run the project.
- [Cairo-lang](https://www.cairo-lang.org/docs/quickstart.html) if you want to do `starknet_call` command.

### Installing

You can get all the dependencies running the next command:

```bash
$ go get -u github.com/NethermindEth/juno 
```

## Running Juno

### Compiling Directly

Compile Juno:

```bash
$ make compile
```

Run Juno:

```bash
$ make run
```

### Using Docker

If you prefer to use docker, you can follow [this](./running/docker.mdx) guide.

---
sidebar_position: 1
---

# Tutorial Intro

Let's discover **Juno in less than 5 min**.

## What you'll need

- [Golang](https://go.dev/doc/install) version 1.18 for build and run the project.
- [Cairo-lang](https://www.cairo-lang.org/docs/quickstart.html) if you want to do `starknet_call` command.

### Installing dependencies

You can get all the dependencies running the next command:

```bash
$ make install-deps
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

If 

### Using Docker

In the other side, if you want to keep your environment clean, and running it using docker, you can
follow [this](../running/docker.mdx) guide.
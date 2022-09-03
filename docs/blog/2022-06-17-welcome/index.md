---
slug: welcome
title: Welcome
authors: [maceo]
tags: [hello, juno]
---

Juno is a Go implementation of a StarkNet full node client made with ❤️ by Nethermind.
## What You'll Need

- [Golang](https://go.dev/doc/install) version 1.18 for build and run the project.
- _For Linux_: You will need to install `clang`:
- Python `3.7`

```shell
sudo apt -y install clang
```

### Installing

#### Install python

We are currently only support python `3.7`, and we recommend use pyenv. To install it, you can follow this instruction:

1. Install dependencies:

```shell
sudo apt-get update; sudo apt-get install make build-essential libssl-dev zlib1g-dev \
libbz2-dev libreadline-dev libsqlite3-dev wget curl llvm \
libncursesw5-dev xz-utils tk-dev libxml2-dev libxmlsec1-dev libffi-dev liblzma-dev

```
2. Install pyenv:

```shell
curl https://pyenv.run | bash
```
3. Add the following entries into your `~/.bashrc` file:

```shell
# pyenv
export PATH="$HOME/.pyenv/bin:$PATH"
eval "$(pyenv init --path)"
eval "$(pyenv virtualenv-init -)"
```
4. Restart your shell:

```shell
exec $SHELL
```
5. Install python 3.7:

```shell
pyenv install 3.7.13
pyenv global 3.7.13
```


#### Installing golang dependencies

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
$ make juno
```

To synchronize with the StarkNet state from the centralized feeder gateway, run the following
command:

```bash
# For Ethereum Goerli testnet
$ ./build/juno

# For Ethereum Mainnet
$ ./build/juno --netowrk 1
```

To sync the state without relying on the feeder gateway, configure an Ethereum node and run the following command:

```bash
# For Ethereum Goerli testnet
$ ./build/juno --eth-node "<node-endpoint>"

# For Ethereum Mainnet
$ ./build/juno --netowrk 1 --eth-node "<node-endpoint>"
```
To view other available options please run `./build/juno -h`.

For more configuration details, check the [config description](/docs/running/config).

### Using Docker

If you prefer to use docker, you can follow [this](/docs/running/docker) guide.

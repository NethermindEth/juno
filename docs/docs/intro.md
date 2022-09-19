---
title: Welcome
sidebar_position: 0
---

# Welcome to Juno

Let's discover **Juno**.

## What You'll Need

- [Golang](https://go.dev/doc/install) version 1.18 for build and run the project.
- Python `3.7`

## Installing

Clone the repository:

```bash
git clone https://github.com/NethermindEth/juno
```

### Install Python dependencies

We are currently only support python `3.7`, and we recommend use pyenv. To install it, you can follow this instruction:

1. Install dependencies:

```shell
sudo apt-get update 
sudo apt-get install make build-essential git patch zlib1g-dev clang \
  openssl libssl-dev libbz2-dev libreadline-dev libsqlite3-dev llvm \
  libncursesw5-dev xz-utils tk-dev libxml2-dev libxmlsec1-dev libffi-dev \
  liblzma-dev
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

6. Inside of the project folder, install python dependencies:

```shell
$ pip install -r requirements.txt
```

### Installing Go dependencies

You can install all golang dependencies running the next command inside the project folder:

```bash
$ go get ./...
```

## Compile

```bash
$ make juno
```

## Run

To synchronize with the StarkNet state from the centralized feeder gateway, run the following
command:

```bash
# For Ethereum Goerli testnet
$ ./build/juno

# For Ethereum Mainnet
$ ./build/juno --network 1
```

To sync the state without relying on the feeder gateway, configure an Ethereum node and run the following command:

```bash
# For Ethereum Goerli testnet
$ ./build/juno --eth-node "<node-endpoint>"

# For Ethereum Mainnet
$ ./build/juno --network 1 --eth-node "<node-endpoint>"
```

To view other available options please run `./build/juno -h`.

For more configuration details, check the [config description](/docs/running/config).

## Using Docker

If you prefer to use docker, you can follow [this](/docs/running/docker) guide.

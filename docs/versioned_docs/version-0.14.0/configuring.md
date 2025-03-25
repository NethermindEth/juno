---
title: Configuring Juno
---

# Configuring Juno :gear:

Juno can be configured using several methods, with the following order of precedence:

1. [Command line parameters (flags)](#command-line-params)
2. [Environment variables](#environment-variables)
3. [Configuration file](#configuration-file)

## Command line params

Juno can be configured directly on the command line by prefixing `--` to each option name:

```bash
./build/juno --http --http-port 6060 --http-host 0.0.0.0 --eth-node <YOUR-ETH-NODE>
```

When using Docker, append the command line parameters after the image name to configure Juno:

```bash
docker run nethermind/juno --http --http-port 6060 --http-host 0.0.0.0 --eth-node <YOUR-ETH-NODE>
```

:::tip
Command line parameters override [environment variables](#environment-variables) and [configuration file](#configuration-file).
:::

## Environment variables

Juno can be configured through environment variables by prefixing the variable names with `JUNO_` and using the configuration options in [SCREAMING_SNAKE_CASE](https://en.wiktionary.org/wiki/screaming_snake_case) format.

To set the `http`, `http-port`, and `http-host` configurations, Juno should be run in this format:

```bash
JUNO_HTTP=true JUNO_HTTP_PORT=6060 JUNO_HTTP_HOST=0.0.0.0 JUNO_ETH_NODE=<YOUR-ETH-NODE> ./build/juno
```

When using Docker, start Juno using the `-e` command option:

```bash
docker run \
  -e "JUNO_HTTP=true JUNO_HTTP_PORT=6060 JUNO_HTTP_HOST=0.0.0.0 JUNO_ETH_NODE=<YOUR-ETH-NODE>" \
  nethermind/juno
```

:::tip
Environment variables rank second in configuration precedence. [Command line parameters](#command-line-params) override environment variables.
:::

## Configuration file

Juno can be configured using a [YAML](https://en.wikipedia.org/wiki/YAML) file:

```yaml title="Sample YAML File" showLineNumbers
log-level: info
network: mainnet
http: true
http-port: 6060
metrics: true
metrics-port: 9090
eth-node: <YOUR-ETH-NODE>
```

To run Juno with a configuration file, use the `config` option to specify the path of the configuration file:

```bash
# Standalone binary
./build/juno --config <CONFIG FILE PATH>

# Docker container
docker run nethermind/juno --config <CONFIG FILE PATH>
```

:::info
By default, Juno looks for the configuration file in the `$XDG_CONFIG_HOME` directory.
:::

:::tip
Configuration file rank third in configuration precedence. [Command line parameters](#command-line-params) and [environment variables](#environment-variables) override configuration file.
:::

## Configuration options

To list all available command line options, you can use the `--help` parameter:

```bash
# Standalone binary
./build/juno --help

# Docker container
docker run nethermind/juno --help
```

Below is a list of all configuration options available in Juno, along with their default values and descriptions:

```mdx-code-block
import ConfigOptions from "./_config-options.md";

<ConfigOptions />
```

## Subcommands

Juno provides several subcommands to perform specific tasks or operations. Here are the available ones:

- `genp2pkeypair`: Generate a private key pair for p2p.
- `db`: Perform database-related operations
  - `db info`: Retrieve information about the database.
  - `db size`: Calculate database size information for each data type.
  - `db revert`: Reverts the database to a specific block number.

To use a subcommand, append it when running Juno:

```bash
# Running a subcommand
./build/juno <subcommand>

# Running the genp2pkeypair subcommand
./build/juno genp2pkeypair

# Running the db info subcommand
./build/juno db info
```

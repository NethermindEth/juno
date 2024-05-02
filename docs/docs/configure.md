---
title: Configure Juno
---

You can configure Juno using a combination of the following methods, listed by priority:

1. [Command line params (flags)](#command-line-params)
2. [Configuration file](#configuration-file)
3. [Default settings](#default-settings)

## Command line params

To configure Juno using command line parameters, prepend `--` to each option name. These parameters will override values set in the configuration file. For example:

```shell
juno --db-path=/juno --http=true --http-port=6060
```

When using Docker, append the command line parameters after the image name to configure Juno:

```shell
docker run nethermind/juno --db-path=/juno --network=mainnet
```

To list all available command line options, you can use the `--help` parameter:

```shell
# Standalone Binaries
juno --help

# Docker Container
docker run nethermind/juno --help
```

## Configuration file

You can configure Juno using a [YAML-formatted](https://en.wikipedia.org/wiki/YAML) configuration file:

```yaml title="Sample YAML File" showLineNumbers
log-level: info
db-path: /juno
network: mainnet
http: true
http-port: 6060
metrics: true
metrics-port: 9090
```

To run Juno with a configuration file, use the `--config` parameter and specify the path of the configuration file:

```shell
# Standalone Binaries
juno --config=[CONFIG FILE PATH]

# Docker Container
docker run nethermind/juno --config=[CONFIG FILE PATH]
```

:::info
By default, Juno searches in the `$XDG_CONFIG_HOME` directory for the configuration file.
:::

## Default settings

Juno runs well with its default settings, removing the need for additional configurations. The `--db-path` and `--http-port` options are enough for basic fine-tuning.

## Configuration options

Below is a list of available configuration options for Juno, along with their default values and descriptions:

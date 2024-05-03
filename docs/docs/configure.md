---
title: Configure Juno
---

Juno can be configured using several methods, with the following order of precedence:

1. [Command line parameters (flags)](#command-line-params)
2. [Configuration file](#configuration-file)
3. [Default settings](#default-settings)

## Command line params

You can set configurations directly on the command line. Prepend `--` to each option name. Command line parameters take precedence over the configuration file:

```shell
juno --db-path=/juno --http=true --http-port=6060
```

When using Docker, append the command line parameters after the image name to configure Juno:

```shell
docker run nethermind/juno --db-path=/juno --network=mainnet
```

## Configuration file

Juno can also be configured using a [YAML](https://en.wikipedia.org/wiki/YAML) file:

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
By default, Juno looks for the configuration file in the `$XDG_CONFIG_HOME` directory.
:::

## Default settings

Juno runs fine with its default settings, which simplifies the configuration process. Setting `--db-path` and `--http-port` may suffice for basic fine-tuning.

To list all available command line options, you can use the `--help` parameter:

```shell
# Standalone Binaries
juno --help

# Docker Container
docker run nethermind/juno --help
```

## Configuration options

Below is a list of all configuration options available in Juno, along with their default values and descriptions:

```mdx-code-block
import ConfigOptions from "./_config-options.md";

<ConfigOptions />
```

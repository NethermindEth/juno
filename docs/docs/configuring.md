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
juno --db-path=/var/lib/juno --network=mainnet
```

When using Docker, append the command line parameters after the image name to configure Juno:

```bash
docker run nethermind/juno --db-path=/var/lib/juno --network=mainnet
```

:::tip
Command line parameters override [environment variables](#environment-variables) and [configuration file](#configuration-file).
:::

## Environment variables

Juno can be configured through environment variables by prefixing the variable names with `JUNO_` and using the configuration options in [SCREAMING_SNAKE_CASE](https://en.wiktionary.org/wiki/screaming_snake_case) format.

To set the `http-port` configuration, Juno should be run in this format:

```bash
JUNO_HTTP_PORT=6060 juno
```

When using Docker, start Juno using the `-e` command option:

```bash
docker run -e "JUNO_HTTP_PORT=6060" nethermind/juno
```

:::tip
Environment variables rank second in configuration precedence. [Command line parameters](#command-line-params) override environment variables.
:::

## Configuration file

Juno can be configured using a [YAML](https://en.wikipedia.org/wiki/YAML) file:

```yaml title="Sample YAML File" showLineNumbers
log-level: info
db-path: /var/lib/juno
network: mainnet
http: true
http-port: 6060
metrics: true
metrics-port: 9090
```

To run Juno with a configuration file, use the `config` option to specify the path of the configuration file:

```bash
# Standalone binary
juno --config=<CONFIG FILE PATH>

# Docker container
docker run nethermind/juno --config=<CONFIG FILE PATH>
```

:::info
By default, Juno looks for the configuration file in the `$XDG_CONFIG_HOME` directory.
:::

:::tip
Configuration file rank third in configuration precedence. [Command line parameters](#command-line-params) and [Environment variables](#environment-variables) override configuration file.
:::

## Configuration options

To list all available command line options, you can use the `--help` parameter:

```bash
# Standalone binary
juno --help

# Docker container
docker run nethermind/juno --help
```

Below is a list of all configuration options available in Juno, along with their default values and descriptions:

```mdx-code-block
import ConfigOptions from "./_config-options.md";

<ConfigOptions />
```

---
slug: /config
sidebar_position: 3
title: Example Configuration
---

The Juno binary uses reasonable defaults and can be used without configuration.
For basic fine-tuning, the `--db-path` and `--http-port` options are usually sufficient.

All available options are in the YAML file below with their default values.
Provide the config using the `--config <filename>` option (Juno looks in `$XDG_CONFIG_HOME` by default).

Juno can also be configured using command line params by prepending `--` to the option name (e.g., `--log-level info`).
Command line params override values in the configuration file. 

```yaml
# Enable colored logs
colour: true

# Path to the database.
# Juno uses `$XDG_DATA_HOME/juno` by default, which is usually something like the value below on Linux.
db-path: /home/<user>/.local/share/juno

# Websocket endpoint of the Ethereum node used to verify the L2 chain.
# If using Infura, it looks something like `wss://mainnet.infura.io/ws/v3/your-infura-project-id`
eth-node: ""

# Port on which the HTTP server will listen for requests.
http-port: 6060

# Options: debug, info, warn, error
log-level: info

# Enable prometheus endpoint.
metrics: false

# Options: mainnet, goerli, goerli2, integration
network: mainnet

# How often to fetch the pending block when synced to the head of the chain.
# Provide a duration like 5s (five seconds) or 10m (10 minutes).
# Disabled by default.
pending-poll-interval: 0s

pprof: false # Enable the pprof endpoint.

# Experimental p2p options; there is currently no standardized Starknet p2p testnet.
p2p: false # Enable the p2p server
p2p-addr: "" # Source address
p2p-boot-peers: "" # Boot nodes
```

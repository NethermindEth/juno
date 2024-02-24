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
# The yaml configuration file
config: ""

# Options: debug, info, warn, error
log-level: info

# Enables the HTTP RPC server on the default port and interface
http: false

# The interface on which the HTTP RPC server will listen for requests
http-host: localhost

# The port on which the HTTP server will listen for requests
http-port: 6060

# Enables the Websocket RPC server on the default port
ws: false

# The interface on which the Websocket RPC server will listen for requests
ws-host: localhost

# The port on which the websocket server will listen for requests
ws-port: 6061

# Location of the database files
db-path: /home/<user>/.local/share/juno

# Options: mainnet, goerli, goerli2, integration, sepolia, sepolia-integration
network: mainnet

# Custom network configuration
custom-network: ""
cn-name: ""
cn-feeder-url: ""
cn-gateway-url: ""
cn-l1-chain-id: ""
cn-l2-chain-id: ""
cn-core-contract-address: ""
cn-unverifiable-range: ""

# Websocket endpoint of the Ethereum node
eth-node: ""

# Enables the pprof endpoint on the default port
pprof: false

# The interface on which the pprof HTTP server will listen for requests
pprof-host: localhost

# The port on which the pprof HTTP server will listen for requests
pprof-port: 6062

# Uses --colour=false command to disable colourized outputs (ANSI Escape Codes)
colour: true

# Sets how frequently pending block will be updated (disabled by default)
pending-poll-interval: 0s

# Enables the prometheus metrics endpoint on the default port
metrics: false

# The interface on which the prometheus endpoint will listen for requests
metrics-host: localhost

# The port on which the prometheus endpoint will listen for requests
metrics-port: 9090

# Enable the HTTP GRPC server on the default port
grpc: false

# The interface on which the GRPC server will listen for requests
grpc-host: localhost

# The port on which the GRPC server will listen for requests
grpc-port: 6064

# Maximum number of VM instances for concurrent RPC calls.
# Default is set to three times the number of CPU cores.
max-vms: 48

# Maximum number of requests to queue for RPC calls after reaching max-vms.
# Default is set to double the value of max-vms.
max-vm-queue: 96

# gRPC URL of a remote Juno node
remote-db: ""

# Maximum number of blocks scanned in single starknet_getEvents call
rpc-max-block-scan: 18446744073709551615

# Determines the amount of memory (in megabytes) allocated for caching data in the database
db-cache-size: 8

# A soft limit on the number of open files that can be used by the DB
db-max-handles: 1024

# API key for gateway/feeder to avoid throttling
gw-api-key: ""

# Maximum number of steps to be executed in starknet_call requests
rpc-call-max-steps: 4000000

# Experimental
# Enable p2p server
p2p: false

# Specify p2p source address as multiaddr
p2p-addr: ""

# Specify list of p2p boot peers splitted by a comma
p2p-boot-peers: ""
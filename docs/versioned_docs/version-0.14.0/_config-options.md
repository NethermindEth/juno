<!-- This file is generated automatically. Any manual modifications will be overwritten. -->

| Config Option | Default Value | Description |
| - | - | - |
| `cn-core-contract-address` |  | Custom network core contract address |
| `cn-feeder-url` |  | Custom network feeder URL |
| `cn-gateway-url` |  | Custom network gateway URL |
| `cn-l1-chain-id` |  | Custom network L1 chain id |
| `cn-l2-chain-id` |  | Custom network L2 chain id |
| `cn-name` |  | Custom network name |
| `cn-unverifiable-range` | `[]` | Custom network range of blocks to skip hash verifications (e.g. `0,100`) |
| `colour` | `true` | Use `--colour=false` command to disable colourized outputs (ANSI Escape Codes) |
| `config` |  | The YAML configuration file |
| `db-cache-size` | `1024` | Determines the amount of memory (in megabytes) allocated for caching data in the database |
| `db-max-handles` | `1024` | A soft limit on the number of open files that can be used by the DB |
| `db-path` | `juno` | Location of the database files |
| `disable-l1-verification` | `false` | Disables L1 verification since an Ethereum node is not provided |
| `eth-node` |  | WebSocket endpoint of the Ethereum node. To verify the correctness of the L2 chain, Juno must connect to an Ethereum node and parse events in the Starknet contract |
| `grpc` | `false` | Enable the HTTP gRPC server on the default port |
| `grpc-host` | `localhost` | The interface on which the gRPC server will listen for requests |
| `grpc-port` | `6064` | The port on which the gRPC server will listen for requests |
| `gw-api-key` |  | API key for gateway endpoints to avoid throttling |
| `gw-timeout` | `5` | Timeout for requests made to the gateway |
| `http` | `false` | Enables the HTTP RPC server on the default port and interface |
| `http-host` | `localhost` | The interface on which the HTTP RPC server will listen for requests |
| `http-port` | `6060` | The port on which the HTTP server will listen for requests |
| `log-level` | `info` | Options: trace, debug, info, warn, error |
| `log-host` | `localhost` | The interface on which the log level HTTP server will listen for requests |
| `log-port` | `0` | The port on which the log level HTTP server will listen for requests |
| `max-vm-queue` | `2 * max-vms` | Maximum number for requests to queue after reaching max-vms before starting to reject incoming requests |
| `max-vms` | `3 * CPU Cores` | Maximum number for VM instances to be used for RPC calls concurrently |
| `metrics` | `false` | Enables the Prometheus metrics endpoint on the default port |
| `metrics-host` | `localhost` | The interface on which the Prometheus endpoint will listen for requests |
| `metrics-port` | `9090` | The port on which the Prometheus endpoint will listen for requests |
| `network` | `mainnet` | Options: mainnet, sepolia, sepolia-integration |
| `p2p` | `false` | EXPERIMENTAL: Enables p2p server |
| `p2p-addr` |  | EXPERIMENTAL: Specify p2p listening source address as multiaddr.  Example: /ip4/0.0.0.0/tcp/7777 |
| `p2p-feeder-node` | `false` | EXPERIMENTAL: Run juno as a feeder node which will only sync from feeder gateway and gossip the new blocks to the network |
| `p2p-peers` |  | EXPERIMENTAL: Specify list of p2p peers split by a comma. These peers can be either Feeder or regular nodes |
| `p2p-private-key` |  | EXPERIMENTAL: Hexadecimal representation of a private key on the Ed25519 elliptic curve |
| `p2p-public-addr` |  | EXPERIMENTAL: Specify p2p public address as multiaddr.  Example: /ip4/35.243.XXX.XXX/tcp/7777 |
| `pending-poll-interval` | `5` | Sets how frequently pending block will be updated (0s will disable fetching of pending block) |
| `plugin-path` |  | Path to the plugin .so file |
| `pprof` | `false` | Enables the pprof endpoint on the default port |
| `pprof-host` | `localhost` | The interface on which the pprof HTTP server will listen for requests |
| `pprof-port` | `6062` | The port on which the pprof HTTP server will listen for requests |
| `remote-db` |  | gRPC URL of a remote Juno node |
| `rpc-call-max-steps` | `4000000` | Maximum number of steps to be executed in starknet_call requests. The upper limit is 4 million steps, and any higher value will still be capped at 4 million |
| `rpc-cors-enable` | `false` | Enable CORS on RPC endpoints |
| `rpc-max-block-scan` | `18446744073709551615` | Maximum number of blocks scanned in single starknet_getEvents call |
| `versioned-constants-file` |  | Use custom versioned constants from provided file |
| `ws` | `false` | Enables the WebSocket RPC server on the default port |
| `ws-host` | `localhost` | The interface on which the WebSocket RPC server will listen for requests |
| `ws-port` | `6061` | The port on which the WebSocket server will listen for requests |

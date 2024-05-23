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
| `db-cache-size` | `8` | Determines the amount of memory (in megabytes) allocated for caching data in the database |
| `db-max-handles` | `1024` | A soft limit on the number of open files that can be used by the DB |
| `db-path` | `juno` | Location of the database files |
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
| `max-vm-queue` |  | Maximum number for requests to queue after reaching max-vms before starting to reject incoming requests. Default is set to double the value of `max-vms` |
| `max-vms` |  | Maximum number for VM instances to be used for RPC calls concurrently. Default is set to three times the number of CPU cores |
| `metrics` | `false` | Enables the Prometheus metrics endpoint on the default port |
| `metrics-host` | `localhost` | The interface on which the Prometheus endpoint will listen for requests |
| `metrics-port` | `9090` | The port on which the Prometheus endpoint will listen for requests |
| `network` | `mainnet` | Options: mainnet, sepolia, sepolia-integration |
| `p2p` | `false` | EXPERIMENTAL: Enables p2p server |
| `p2p-addr` |  | EXPERIMENTAL: Specify p2p source address as multiaddr |
| `p2p-feeder-node` | `false` | EXPERIMENTAL: Run juno as a feeder node which will only sync from feeder gateway and gossip the new blocks to the network |
| `p2p-peers` |  | EXPERIMENTAL: Specify list of p2p peers split by a comma. These peers can be either Feeder or regular nodes |
| `p2p-private-key` |  | EXPERIMENTAL: Hexadecimal representation of a private key on the Ed25519 elliptic curve |
| `pending-poll-interval` | `5` | Sets how frequently pending block will be updated (0s will disable fetching of pending block) |
| `pprof` | `false` | Enables the pprof endpoint on the default port |
| `pprof-host` | `localhost` | The interface on which the pprof HTTP server will listen for requests |
| `pprof-port` | `6062` | The port on which the pprof HTTP server will listen for requests |
| `remote-db` |  | gRPC URL of a remote Juno node |
| `rpc-call-max-steps` | `4000000` | Maximum number of steps to be executed in starknet_call requests |
| `rpc-cors-enable` | `false` | Enable CORS on RPC endpoints |
| `rpc-max-block-scan` | `18446744073709551615` | Maximum number of blocks scanned in single starknet_getEvents call |
| `ws` | `false` | Enables the WebSocket RPC server on the default port |
| `ws-host` | `localhost` | The interface on which the WebSocket RPC server will listen for requests |
| `ws-port` | `6061` | The port on which the WebSocket server will listen for requests |

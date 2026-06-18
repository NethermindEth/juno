<!-- This file is generated automatically. Any manual modifications will be overwritten. -->

### HTTP RPC

| Config Option | Default Value | Description |
| - | - | - |
| `disable-rpc-batch-requests` | `false` | Disables handling of batched RPC requests |
| `http` | `false` | Enables the HTTP RPC server on the default port and interface |
| `http-host` | `localhost` | The interface on which the HTTP RPC server will listen for requests |
| `http-port` | `6060` | The port on which the HTTP server will listen for requests |
| `rpc-call-max-gas` | `100000000` | Maximum number of Sierra gas to be executed in starknet_call requests |
| `rpc-call-max-steps` | `4000000` | Maximum number of steps to be executed in starknet_call requests |
| `rpc-cors-enable` | `false` | Enable CORS on RPC endpoints |
| `rpc-max-block-scan` | `18446744073709551615` | Maximum number of blocks scanned in single starknet_getEvents call |
| `rpc-max-concurrent-requests` | `256000` | Maximum concurrent HTTP RPC requests; 0 disables the limit |
| `rpc-max-request-queue` | `256000` | Maximum number of HTTP RPC requests to queue after reaching rpc-max-concurrent-requests before rejecting incoming requests |
| `rpc-request-timeout` | `1m` | Maximum time for an RPC request to complete |

### WebSocket RPC

| Config Option | Default Value | Description |
| - | - | - |
| `disable-received-txn-stream` | `false` | The starknet_subscribeNewTransactions WebSocket API allows users to subscribe to new transactions. By default, it streams transactions that have been accepted on L2. Users can optionally provide a set of finality statuses to be notified about, including transactions from canonical blocks, blocks with softer finality guarantees such as pre-confirmed and pre-latest, as well as transactions not yet part of any block such as received and candidate. When subscribers select the RECEIVED status, they will be notified about transactions that have been submitted through this node — these transactions are local to the node and are not sourced from the network. When this flag is enabled, the node will no longer notify subscribers about transactions submitted through it |
| `ws` | `false` | Enables the WebSocket RPC server on the default port |
| `ws-host` | `localhost` | The interface on which the WebSocket RPC server will listen for requests |
| `ws-port` | `6061` | The port on which the WebSocket server will listen for requests |

### Network & L1

| Config Option | Default Value | Description |
| - | - | - |
| `disable-l1-verification` | `false` | Disables L1 verification since an Ethereum node is not provided |
| `eth-node` |  | WebSocket endpoint of the Ethereum node. To verify the correctness of the L2 chain, Juno must connect to an Ethereum node and parse events in the Starknet contract |
| `network` | `mainnet` | Options: mainnet, sepolia, sepolia-integration |

### Sync & Polling

| Config Option | Default Value | Description |
| - | - | - |
| `preconfirmed-poll-interval` | `500ms` | Sets how frequently pre_confirmed block will be updated(0s will disable fetching of pre_confirmed block) |
| `readiness-block-tolerance` | `6` | Maximum blocks behind latest for /ready endpoints to return 200 OK |
| `remote-db` |  | gRPC URL of a remote Juno node |

### Gateway

| Config Option | Default Value | Description |
| - | - | - |
| `gw-api-key` |  | API key for gateway endpoints to avoid throttling |
| `gw-timeouts` | `5s` | Timeouts for requests made to the gateway. Can be specified in three ways:\n- Single value (e.g. '5s'): After each failure, the timeout will increase dynamically.\n- Comma-separated list (e.g. '5s,10s,20s'): Each value will be used in sequence after failures.\n- Single value with trailing comma (e.g. '5s,'): Uses a fixed timeout without dynamic adjustment |

### Pruning

| Config Option | Default Value | Description |
| - | - | - |
| `prune-min-age` | `1h` | Protect blocks whose on-chain timestamp is younger than this duration from being pruned. Acts as an additional floor on top of --prune-mode: a block is retained if either the block-count window or this minimum-age window covers it. Set 0 to disable. Default 1h. Requires --prune-mode |
| `prune-mode` | `128` | Enables block-data and state-history pruning. Pruning is disabled by default; passing this flag (with or without a value) turns it on. The value is the size of the retention window in blocks, counted back from the retention pivot (the lower of the L1-verified head and the local L2 head):\n  --prune-mode      same as --prune-mode=128; keep 128 blocks below the pivot\n  --prune-mode=N    keep blocks in [pivot - N, l2_head], prune below\nBlocks at or above the L2 head are always kept. The pivot is at or below the L1-verified head, so pruned blocks are reorg-safe. RPC remains fully functional for any block inside the retention window; requests targeting blocks below the floor fail because their data has been deleted. Pruning is irreversible: data deleted under a small window cannot be recovered without re-syncing. Changing this value across restarts is safe: the window grows or shrinks accordingly. Growth is gradual — pruning pauses until the pivot advances enough to reach the new floor |

### Logging

| Config Option | Default Value | Description |
| - | - | - |
| `colour` | `true` | Use `--colour=false` command to disable colourized outputs (ANSI Escape Codes) |
| `log-json` | `false` | Use JSON encoding for log output |
| `log-level` |  | Options: trace, debug, info, warn, error |

### Logs HTTP Update Endpoint

| Config Option | Default Value | Description |
| - | - | - |
| `http-update-host` | `localhost` | The interface on which the log level and gateway timeouts HTTP server will listen for requests |
| `http-update-port` | `0` | The port on which the log level and gateway timeouts HTTP server will listen for requests |

### Metrics

| Config Option | Default Value | Description |
| - | - | - |
| `metrics` | `false` | Enables the Prometheus metrics endpoint on the default port |
| `metrics-host` | `localhost` | The interface on which the Prometheus endpoint will listen for requests |
| `metrics-port` | `9090` | The port on which the Prometheus endpoint will listen for requests |

### Database

| Config Option | Default Value | Description |
| - | - | - |
| `db-cache-size` | `1024` | Determines the amount of memory (in megabytes) allocated for caching data in the database |
| `db-compaction-concurrency` |  | DB compaction concurrency range. Format: N (lower=1, upper=N) or M,N (lower=M, upper=N). Default: 1,GOMAXPROCS/2 |
| `db-compression` | `zstd` | Database compression profile. Options: zstd, snappy, minlz. Use zstd for low storage |
| `db-max-handles` | `1024` | A soft limit on the number of open files that can be used by the DB |
| `db-memtable-count` | `2` | Determines the number of memtables the database can queue before stalling writes |
| `db-memtable-size` | `256` | Determines the amount of memory (in MBs) allocated for database memtables |
| `db-path` | `juno` | Location of the database files |

### Transaction Cache

| Config Option | Default Value | Description |
| - | - | - |
| `submitted-transactions-cache-entry-ttl` | `5m` | Time-to-live for each entry in the submitted transactions cache |
| `submitted-transactions-cache-size` | `10000` | Maximum number of entries in the submitted transactions cache |

### VM & Compilation

| Config Option | Default Value | Description |
| - | - | - |
| `max-compilation-cpu-time` | `10` | Maximum CPU time (in seconds) each Sierra compilation process may consume; a compilation exceeding it is aborted. Enforced on Linux only. 0 disables the limit |
| `max-compilation-memory` | `4 * 1024` | Maximum memory (in MB) each Sierra compilation process may use; a compilation exceeding it is aborted. Enforced on Linux only. 0 disables the limit |
| `max-compilation-queue` | `2 * max-concurrent-compilations` | Maximum number of compilation requests to queue after reaching max-concurrent-compilations before starting to reject incoming requests |
| `max-concurrent-compilations` | `CPU Cores` | Maximum concurrent Sierra compilations |
| `max-vm-queue` | `2 * max-vms` | Maximum number for requests to queue after reaching max-vms before starting to reject incoming requests |
| `max-vms` | `3 * CPU Cores` | Maximum number for VM instances to be used for RPC calls concurrently |
| `versioned-constants-file` |  | Use custom versioned constants from provided file |

### Custom Network

| Config Option | Default Value | Description |
| - | - | - |
| `cn-core-contract-address` |  | Custom network core contract address |
| `cn-feeder-url` |  | Custom network feeder URL |
| `cn-gateway-url` |  | Custom network gateway URL |
| `cn-l1-chain-id` |  | Custom network L1 chain id |
| `cn-l2-chain-id` |  | Custom network L2 chain id |
| `cn-name` |  | Custom network name |
| `cn-unverifiable-range` | `[]` | Custom network range of blocks to skip hash verifications (e.g. `0,100`) |

### Profiling

| Config Option | Default Value | Description |
| - | - | - |
| `pprof` | `false` | Enables the pprof endpoint on the default port |
| `pprof-host` | `localhost` | The interface on which the pprof HTTP server will listen for requests |
| `pprof-port` | `6062` | The port on which the pprof HTTP server will listen for requests |

### P2P (experimental)

| Config Option | Default Value | Description |
| - | - | - |
| `p2p` | `false` | EXPERIMENTAL: Enables p2p server |
| `p2p-addr` |  | EXPERIMENTAL: Specify p2p listening source address as multiaddr.  Example: /ip4/0.0.0.0/tcp/7777 |
| `p2p-feeder-node` | `false` | EXPERIMENTAL: Run juno as a feeder node which will only sync from feeder gateway and gossip the new blocks to the network |
| `p2p-peers` |  | EXPERIMENTAL: Specify list of p2p peers split by a comma. These peers can be either Feeder or regular nodes |
| `p2p-private-key` |  | EXPERIMENTAL: Hexadecimal representation of a private key on the Ed25519 elliptic curve |
| `p2p-public-addr` |  | EXPERIMENTAL: Specify p2p public address as multiaddr.  Example: /ip4/35.243.XXX.XXX/tcp/7777 |

### Sequencer

| Config Option | Default Value | Description |
| - | - | - |
| `seq-block-time` | `60` | Time to build a block, in seconds |
| `seq-disable-fees` | `false` | Skip charge fee for sequencer execution |
| `seq-enable` | `false` | Enables sequencer mode of operation |
| `seq-genesis-file` |  | Path to the genesis file |

### gRPC

| Config Option | Default Value | Description |
| - | - | - |
| `grpc` | `false` | Enable the HTTP gRPC server on the default port |
| `grpc-host` | `localhost` | The interface on which the gRPC server will listen for requests |
| `grpc-port` | `6064` | The port on which the gRPC server will listen for requests |

### Plugins & Misc

| Config Option | Default Value | Description |
| - | - | - |
| `config` |  | The YAML configuration file |
| `new-state` | `false` | EXPERIMENTAL: Use the new state package implementation |
| `plugin-path` |  | Path to the plugin .so file |

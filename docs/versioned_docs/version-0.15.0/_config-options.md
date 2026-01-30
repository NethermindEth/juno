<!-- This file is generated automatically. Any manual modifications will be overwritten. -->

| Config Option | Default Value | Description |
| --- | --- | --- |
| `cn-core-contract-address` |  | Custom network core contract address. |
| `cn-feeder-url` |  | Custom network feeder URL. |
| `cn-gateway-url` |  | Custom network gateway URL. |
| `cn-l1-chain-id` |  | Custom network L1 chain id. |
| `cn-l2-chain-id` |  | Custom network L2 chain id. |
| `cn-name` |  | Custom network name. |
| `cn-unverifiable-range` | `0,100` | Custom network range of blocks to skip hash verifications (e.g. 0,100). |
| `colour` | `true` | Use `--colour=false` command to disable colourized outputs (ANSI Escape Codes). |
| `config` |  | The YAML configuration file. |
| `db-cache-size` | `1024` | Determines the amount of memory (in megabytes) allocated for caching data in the database. |
| `db-compaction-concurrency` |  | DB compaction concurrency range. Format: N (lower=1, upper=N) or M,N (lower=M, upper=N). Default: 1,GOMAXPROCS/2 |
| `db-compression` | `"snappy"` | Database compression profile. Options: snappy, zstd, minlz. Use zstd for low storage. |
| `db-max-handles` | `1024` | A soft limit on the number of open files that can be used by the DB |
| `db-memtable-size` | `256` | Determines the amount of memory (in MBs) allocated for database memtables. |
| `db-path` | `path to your db location` | Location of the database files. |
| `disable-l1-verification` |  | Disables L1 verification since an Ethereum node is not provided. |
| `disable-rpc-batch-requests` |  | Disables handling of batched RPC requests. |
| `eth-node` |  | WebSocket endpoint of the Ethereum node. To verify the correctness of the L2 chain, Juno must connect to an Ethereum node and parse events in the Starknet contract. |
| `grpc` |  | Enable the HTTP gRPC server on the default port. |
| `grpc-host` | `"localhost"` | The interface on which the gRPC server will listen for requests. |
| `grpc-port` | `6064` | The port on which the gRPC server will listen for requests. |
| `gw-api-key` |  | API key for gateway endpoints to avoid throttling |
| `gw-timeouts` | `"5s"` | Timeouts for requests made to the gateway. Can be specified in three ways: - Single value (e.g. '5s'): After each failure, the timeout will increase dynamically. - Comma-separated list (e.g. '5s,10s,20s'): Each value will be used in sequence after failures. - Single value with trailing comma (e.g. '5s,'): Uses a fixed timeout without dynamic adjustment. |
| `help` |  | help for juno |
| `http` |  | Enables the HTTP RPC server on the default port and interface. |
| `http-host` | `"localhost"` | The interface on which the HTTP RPC server will listen for requests. |
| `http-port` | `6060` | The port on which the HTTP server will listen for requests. |
| `http-update-host` | `"localhost"` | The interface on which the log level and gateway timeouts HTTP server will listen for requests. |
| `http-update-port` |  | The port on which the log level and gateway timeouts HTTP server will listen for requests. |
| `log-level` | `"info"` | Options: trace, debug, info, warn, error. |
| `max-vm-queue` | `72` | Maximum number for requests to queue after reaching max-vms before starting to reject incoming requests |
| `max-vms` | `36` | Maximum number for VM instances to be used for RPC calls concurrently |
| `metrics` |  | Enables the Prometheus metrics endpoint on the default port. |
| `metrics-host` | `"localhost"` | The interface on which the Prometheus endpoint will listen for requests. |
| `metrics-port` | `9090` | The port on which the Prometheus endpoint will listen for requests. |
| `network` | `mainnet` | Options: mainnet, sepolia, sepolia-integration. |
| `p2p` |  | EXPERIMENTAL: Enables p2p server. |
| `p2p-addr` |  | EXPERIMENTAL: Specify p2p listening source address as multiaddr. Example: /ip4/0.0.0.0/tcp/7777 |
| `p2p-feeder-node` |  | EXPERIMENTAL: Run juno as a feeder node which will only sync from feeder gateway and gossip the new blocks to the network. |
| `p2p-peers` |  | EXPERIMENTAL: Specify list of p2p peers split by a comma. These peers can be either Feeder or regular nodes. |
| `p2p-private-key` |  | EXPERIMENTAL: Hexadecimal representation of a private key on the Ed25519 elliptic curve. |
| `p2p-public-addr` |  | EXPERIMENTAL: Specify p2p public address as multiaddr. Example: /ip4/35.243.XXX.XXX/tcp/7777 |
| `pending-poll-interval` | `1s` | Sets polling interval for pending block updates before starknet v0.14.0;for pre_latest block updates from starknet v0.14.0 onward.(0s will disable polling). |
| `plugin-path` |  | Path to the plugin .so file |
| `pprof` |  | Enables the pprof endpoint on the default port. |
| `pprof-host` | `"localhost"` | The interface on which the pprof HTTP server will listen for requests. |
| `pprof-port` | `6062` | The port on which the pprof HTTP server will listen for requests. |
| `preconfirmed-poll-interval` | `500ms` | Sets how frequently pre_confirmed block will be updated(0s will disable fetching of pre_confirmed block). |
| `readiness-block-tolerance` | `6` | Maximum blocks behind latest for /ready endpoints to return 200 OK |
| `remote-db` |  | gRPC URL of a remote Juno node |
| `rpc-call-max-gas` | `100000000` | Maximum number of Sierra gas to be executed in starknet_call requests |
| `rpc-call-max-steps` | `4000000` | Maximum number of steps to be executed in starknet_call requests |
| `rpc-cors-enable` |  | Enable CORS on RPC endpoints |
| `rpc-max-block-scan` | `18446744073709551615` | Maximum number of blocks scanned in single starknet_getEvents call |
| `seq-block-time` | `60` | Time to build a block, in seconds |
| `seq-disable-fees` |  | Skip charge fee for sequencer execution |
| `seq-enable` |  | Enables sequencer mode of operation |
| `seq-genesis-file` |  | Path to the genesis file |
| `submitted-transactions-cache-entry-ttl` | `5m0s` | Time-to-live for each entry in the submitted transactions cache |
| `submitted-transactions-cache-size` | `10000` | Maximum number of entries in the submitted transactions cache |
| `transaction-combined-layout` |  | EXPERIMENTAL: Enable combined (per-block) transaction storage layout. Once enabled, cannot be disabled. |
| `version` |  | version for juno |
| `versioned-constants-file` |  | Use custom versioned constants from provided file |
| `ws` |  | Enables the WebSocket RPC server on the default port. |
| `ws-host` | `"localhost"` | The interface on which the WebSocket RPC server will listen for requests. |
| `ws-port` | `6061` | The port on which the WebSocket server will listen for requests. |          |

# Sync benchmark: feeder-sim

Captures feeder gateway (FGW) responses for a block range, then serves them from memory behind a
simulated chain tip. Removes FGW latency, rate limits and chain activity from sync benchmarks.

## Use

```sh
make feeder-sim                                                                   # -> build/feeder-sim
build/feeder-sim --data ./data/mainnet --network mainnet --from 1500000 --to 1501000 --listen off  # capture
build/feeder-sim --data ./data/mainnet --from 1500000 --to 1501000                # serve, 2s cadence
build/feeder-sim --data ./data/mainnet --from 1500000 --to 1501000 --speed 1      # serve, captured block times
build/feeder-sim --data ./data/mainnet --from 1500000 --to 1501000 --tip 1501000  # serve everything at once
build/feeder-sim --data ./data/mainnet --network mainnet --rpc-url http://node:6060 --from 1500000 --to 1501000 --listen off --preconfirmed  # capture pre-confirmed too
build/feeder-sim --data ./data/mainnet --from 1500000 --to 1501000 --preconfirmed        # serve with get_preconfirmed_block
```

Capture is resumable. A 400 from the FGW usually means `--to` is above the tip. One `--data`
directory per network, checked via `get_contract_addresses`. Without `--network` the sim is offline and fails on a missing file; with
`--network` it fills gaps before serving.

## Flags

| Flag                    | Meaning                                                                             | Default          |
| ----------------------- | ----------------------------------------------------------------------------------- | ---------------- |
| `--data`                | dataset directory (required)                                                        |                  |
| `--from`, `--to`        | block range, inclusive (required)                                                   |                  |
| `--network`             | capture source: `mainnet`, `sepolia`, `sepolia-integration`                         | offline          |
| `--listen`              | `host:port`; `:7070` binds all interfaces; `off` = capture only                     | `127.0.0.1:7070` |
| `--tip`                 | initial tip, in `[from, to]`                                                        | `from`           |
| `--interval`            | advance the tip by one block every interval                                         | `2s`             |
| `--speed`               | replay captured block timestamps at this multiplier; excludes `--interval`          | unset            |
| `--latency`             | fixed delay added to every response, no jitter                                      | `0`              |
| `--api-key`             | `X-Throttling-Bypass` header during capture                                         |                  |
| `--concurrency`         | parallel capture requests                                                           | `8`              |
| `--capture-timeout`     | per-request timeout during capture                                                  | `30s`            |
| `--capture-retries`     | retries per request during capture                                                  | `5`              |
| `--preconfirmed`        | capture and serve `get_preconfirmed_block`                                          | off              |
| `--rpc-url`             | JSON-RPC node with `starknet_traceBlockTransactions`; `--preconfirmed` capture only |                  |
| `--preconfirmed-lead`   | pre-confirmed blocks served above the tip, `>= 1`                                   | `3`              |
| `--preconfirmed-keep`   | pre-confirmed blocks kept below the tip                                             | `5`              |
| `--preconfirmed-stages` | steps in which the top block reveals its transactions; `0` = all at once            | `3`              |
| `--log-level`           | `debug`, `info`, `warn`, `error`                                                    | `info`           |

`blockNumber=latest` resolves to the tip. Blocks above the tip get 400 (debug log). Classes are not tip-gated.

`--preconfirmed` serves `get_preconfirmed_block` for `[tip-keep, tip+lead]`; only `tip+lead` fills, in
`stages+1` steps per interval, and `latest` is that block. `blockIdentifier` is required. Like the FGW: a mismatched
`blockIdentifier` gets a full block; otherwise a `knownTransactionCount` at or past the revealed transactions gets
`{"changed": false}`, a count of 0 gets a full block, and any other count gets a delta since the count. Blocks before
Starknet 0.13.1 lack `l2_gas_price` and fail capture.

## Juno

```sh
juno --db-path <copy of a DB synced to from-1> \
  --cn-name mainnet --cn-feeder-url http://127.0.0.1:7070/feeder_gateway/ \
  --cn-gateway-url http://127.0.0.1:7070/gateway/ \
  --cn-l2-chain-id SN_MAIN --cn-l1-chain-id 1 \
  --cn-core-contract-address 0xc662c410c0ecf747543f5ba90660f6abebd9c8c4 \
  --cn-unverifiable-range 0,0 --preconfirmed-poll-interval 0 --metrics
```

- Drop `--preconfirmed-poll-interval 0` when the sim runs with `--preconfirmed`; the capture log prints the right flags.
- Restore the DB copy before every run. To prepare a DB at `X-1`, run against the sim with `--tip X-1`.
- Keep the range at Starknet v0.13.2 or later; older blocks fail hash verification on custom networks.
- Juno retries a 400 ten times with backoff, so wall clock between blocks measures the retry
  schedule. Measure `sync_step_duration` instead, or use `--tip <to>` for throughput.

## Dataset layout

One gzipped file per FGW response:

```
<data>/
  get_contract_addresses.json.gz
  get_block/<N>.json.gz                           # headerOnly=true
  get_state_update/<N>.json.gz                    # includeBlock=true&includeSignature=true
  get_class_by_hash/<hash>.json.gz                # blockNumber=latest
  get_compiled_class_by_class_hash/<hash>.json.gz # blockNumber=latest
  get_preconfirmed_block/<N>.json.gz              # --preconfirmed; completed round, diffs from --rpc-url
```

Timestamps, class lists and completeness are derived from the state updates at startup.

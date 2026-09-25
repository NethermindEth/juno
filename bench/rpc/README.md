# RPC benchmark harness (`bench/rpc`)

Sample a Starknet node into a seeded JSON-RPC corpus, then replay it with
[k6](https://k6.io) to measure latency and throughput.

## Methods

Run `./build/corpus-gen --help` for the full list. One subcommand per method
(`starknet_` prefix dropped), covering the read, trace and execution APIs plus
`starknet_getCompiledCasm`; the write and websocket APIs,
`starknet_estimateMessageFee` and `starknet_getMessagesStatus` are not
generated.

Method-specific flags:

| Subcommand        | Flag                                        | Meaning                                            |
| ----------------- | ------------------------------------------- | -------------------------------------------------- |
| block-id subcommands | `--block-id`                             | `block_id` encoding: `number` (default), `hash` or `latest` |
| `getBlockWithTxs`/`getBlockWithReceipts`/`getTransactionByHash`/`getTransactionByBlockIdAndIndex` | `--include-proof-facts` | add `INCLUDE_PROOF_FACTS` to `response_flags` |
| `getStorageAt`    | `--include-last-update-block`               | add `INCLUDE_LAST_UPDATE_BLOCK` to `response_flags` |
| `traceBlockTransactions` | `--return-initial-reads`             | add `RETURN_INITIAL_READS` to `trace_flags`        |
| `getEvents`       | `--window`                                  | blocks per event filter, one value or `min,max` uniform draw (default 100); a single `0` omits `from_block`/`to_block` |
| `getEvents`       | `--chunk-size`                              | `chunk_size` per request, one value or `min,max` uniform draw (default 1000) |
| `getEvents`       | `--addresses`                               | emitter addresses in the filter (0 = no address filter) |
| `getEvents`       | `--keys`                                    | key counts per position, e.g. `1,0,2` (0 = wildcard; omit for no keys filter) |
| `getStorageProof` | `--num-classes`/`--num-contracts`/`--num-keys` | trie members per request (queried at `latest`)  |
| `estimateFee`/`simulateTransactions` | `--num-txs`          | broadcasted transactions per request, one value or `min,max` uniform draw (default 1) |
| `estimateFee`/`simulateTransactions` | `--tx-types`         | transaction types to take from the block: `INVOKE`, `DECLARE`, `DEPLOY_ACCOUNT` (default `INVOKE,DEPLOY_ACCOUNT`) |
| `estimateFee`     | `--skip-validate`                           | add `SKIP_VALIDATE` to `simulation_flags`          |
| `simulateTransactions` | `--skip-validate`/`--skip-fee-charge`/`--return-initial-reads` | add those flags to `simulation_flags` |
| execution subcommands | `--verify`                              | replay each sample against the source node and draw again when it fails (default on) |

### Execution methods

`estimateFee`, `simulateTransactions` and `call` sample a block N, then build the
request from the transactions of block N+1, which ran on the state of block N:

- `estimateFee` and `simulateTransactions` send a leading run of block N+1's
  transactions. Real transactions keep their nonces and signatures valid, so
  `SKIP_VALIDATE` stays optional. The run stops at the first transaction with a
  version below `0x3` or a type outside `--tx-types`. A block whose run is
  shorter than `--num-txs` is skipped.
- `call` takes one call out of an account multicall in block N+1.
- `--block-id latest` is not supported, because block N+1 must exist.
- `--block-start` defaults to 10000 blocks below `--block-end`. Older ranges need
  a node that keeps their state, and blocks before the Starknet 0.13.0 upgrade
  carry no `0x3` transactions at all.
- `--verify` keeps the corpus clean: `run.js` counts a JSON-RPC error as a
  failure, and a real transaction can still revert or run short of resources
  when it replays against the gas prices of block N. Generation therefore runs
  execution work on the source node, and `--batch N` multiplies it by N. The
  check only covers the source node: another node can still reject a request
  that `--source-url` accepted.
- `--tx-types` selects the cost class under measurement: `INVOKE` executes,
  `DEPLOY_ACCOUNT` also runs `__validate_deploy__` and a deployment, `DECLARE`
  compiles. It is not a filter over the block: a transaction of any other type
  ends the run, so a narrow list draws more blocks per entry. Watch the
  `sampling attempts` line that generation prints. `L1_HANDLER` and `DEPLOY`
  are rejected, since the execution methods do not take them.
- `DECLARE` sends a full contract class per transaction, which the node compiles
  on every request. The corpus grows by ~100KB-1MB per entry, and the benchmark
  then measures compilation instead of execution.

## Use

### Install

```
make install-k6
make corpus-gen
```

### Full flow

```
./bench/rpc/gen-all.sh bench/rpc/corpus/all.json juno
./bench/rpc/run-all.sh bench/rpc/corpus/all.json juno --vus 500 --duration 30s
```

Both take `<corpus>` (config or its folder, `all.json` ↔ `all/`) and `<node>`
(name in `nodes.json` or a literal URL); remaining args pass through.

`gen-all.sh`: one corpus per config entry (`{"name": "subcommand [flags]"}`)
into the config's folder, sampling `<node>` via `--source-url`; per-entry
flags win.

`run-all.sh`: `run.js` once per corpus against `<node>`. Writes
`<corpus>/<node>/` (overwritten per re-run): `<method>.html` (dashboard, live
at `:5665`), `<method>.json` (summary), `report.md`. Failures don't stop the
sweep.

### corpus-gen

```
mkdir -p bench/rpc/corpus/v0_10
./build/corpus-gen getTransactionByHash --count 10000 --block-start 0 --block-end 12000000 --seed 1 > bench/rpc/corpus/v0_10/getTransactionByHash.json
```

### Closed model

Shared iterations — sequential latency:

```
k6 run bench/rpc/run.js -e NODE_URL=http://localhost:6060/v0_10 --vus 1 --iterations 200 < bench/rpc/corpus/v0_10/getTransactionByHash.json
```

Constant VUs — fixed concurrency:

```
k6 run bench/rpc/run.js -e NODE_URL=http://localhost:6060/v0_10 --vus 50 --duration 30s < bench/rpc/corpus/v0_10/getTransactionByHash.json
```

### Open model

Ramping arrival rate — fixed offered load, req/s:

```
k6 run bench/rpc/throughput.js -e NODE_URL=http://localhost:6060/v0_10 -e RATES=1000,2000,3000 -e DURATION=5s < bench/rpc/corpus/v0_10/getTransactionByHash.json
```

Corpus must be redirected in (`<`), not piped. k6 prints its summary to stdout;
`--summary-export` additionally writes it as JSON. Add
`--summary-trend-stats "avg,min,med,p(90),p(99),max"` to include p99.

## corpus-gen flags

| Flag            | Meaning                                        |
| --------------- | ---------------------------------------------- |
| `--count`       | corpus entries                                 |
| `--block-start` | sample range low (inclusive, default 0)        |
| `--block-end`   | sample range high (inclusive, default latest)  |
| `--seed`        | reproducible corpus                            |
| `--batch N`     | N requests per entry                           |
| `--concurrency` | concurrent sampling requests (`GOMAXPROCS`)    |
| `--source-url`  | node to sample (`http://localhost:6060/v0_10`) |

## k6 flags & env

| Knob                            | Applies to             | Meaning                                                       |
| ------------------------------- | ---------------------- | ------------------------------------------------------------- |
| `-e NODE_URL=<url>`             | both                   | endpoint under test (required)                                |
| `--summary-export <file>`       | both                   | write the end-of-test summary to a JSON file                  |
| `--summary-trend-stats <stats>` | both                   | trend stats, e.g. `"avg,min,med,p(90),p(99),max"`             |
| `--vus <n>`                     | closed (`run.js`)      | concurrent VUs (`1` = sequential)                             |
| `--iterations <n>`              | closed (`run.js`)      | stop after this many requests                                 |
| `--duration <time>`             | closed (`run.js`)      | stop after this run length (`30s`)                            |
| `-e RATES=<r1,r2,...>`          | open (`throughput.js`) | ramp targets, req/s (`1000,2000,3000`)                        |
| `-e DURATION=<time>`            | open (`throughput.js`) | per-stage ramp time (`5s`)                                    |

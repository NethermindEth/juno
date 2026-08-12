# RPC benchmark harness (`bench/rpc`)

Sample a Starknet node into a seeded JSON-RPC corpus, then replay it with
[k6](https://k6.io) to measure latency and throughput.

## Methods

- `starknet_getTransactionByHash`
- `starknet_getTransactionReceipt`
- `starknet_getBlockWithTxs`
- `starknet_getBlockWithTxHashes`
- `starknet_getBlockWithReceipts`


## Use

### Install

```
make install-k6
make corpus-gen
```

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
| `--block-start` | sample range low (inclusive)                   |
| `--block-end`   | sample range high (exclusive)                  |
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

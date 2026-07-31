# RPC benchmark harness (`bench/rpc`)

Sample a Starknet node into a seeded JSON-RPC corpus, then replay it with
[k6](https://k6.io) to measure latency and throughput.

## Methods

- `starknet_getTransactionByHash`

## Use

```
make install-k6
make corpus-gen
mkdir -p bench/rpc/corpus/v0_10
./build/corpus-gen getTransactionByHash \
    --count 1000 --block-start 0 --block-end 800000 --seed 1 \
    > bench/rpc/corpus/v0_10/getTransactionByHash.json
k6 run bench/rpc/single.js -e NODE_URL=http://localhost:6060/v0_10 \
    < bench/rpc/corpus/v0_10/getTransactionByHash.json > result.json
```

Scenarios: `single.js`, `concurrency.js`, `throughput.js` (share `common.js`).
Corpus must be redirected in (`<`), not piped. Summary → stdout, table → stderr.

## corpus-gen flags

| Flag            | Meaning                                        |
| --------------- | ---------------------------------------------- |
| `--count`       | corpus entries                                 |
| `--block-start` | sample range low (inclusive)                   |
| `--block-end`   | sample range high (exclusive)                  |
| `--seed`        | reproducible corpus                            |
| `--batch N`     | N requests per entry                           |
| `--source-url`  | node to sample (`http://localhost:6060/v0_10`) |

## k6 env

| Env          | Meaning                                       |
| ------------ | --------------------------------------------- |
| `NODE_URL`   | endpoint under test (required)                |
| `VUS`        | concurrent VUs (`50`)                         |
| `ITERATIONS` | requests, single scenario (`200`)             |
| `RATES`      | throughput ramp targets, req/s (`50,100,200`) |
| `DURATION`   | run length / per-stage ramp time (`30s`)      |

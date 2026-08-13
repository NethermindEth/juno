# RPC benchmark harness (`bench/rpc`)

Sample a Starknet node into a seeded JSON-RPC corpus, then replay it with
[k6](https://k6.io) to measure latency and throughput.

## Methods

Run `./build/corpus-gen --help` for the full list. One subcommand per method
(`starknet_` prefix dropped), covering the read and trace APIs plus
`starknet_getCompiledCasm`; the write and websocket APIs and the execution
methods (`starknet_call`, `starknet_estimateFee`, `starknet_estimateMessageFee`,
`starknet_simulateTransactions`, `starknet_getMessagesStatus`) are not
generated.

Method-specific flags:

| Subcommand        | Flag                                        | Meaning                                            |
| ----------------- | ------------------------------------------- | -------------------------------------------------- |
| `getEvents`       | `--max-window`                              | max blocks per event filter                        |
| `getEvents`       | `--chunk-size`                              | `chunk_size` per request                           |
| `getEvents`       | `--address-prob`                            | probability of filtering by an emitting address    |
| `getStorageProof` | `--num-classes`/`--num-contracts`/`--num-keys` | trie members per request (queried at `latest`)  |

## Use

### Install

```
make install-k6
make corpus-gen
```

### corpus-gen

```
mkdir -p bench/rpc/corpus/v0_10
./build/corpus-gen getTransactionByHash --source-url http://localhost:6060/v0_10 --count 10000 --block-start 0 --block-end 12945536 --seed 1 > bench/rpc/corpus/v0_10/getTransactionByHash.json
```

The committed baseline corpus targets mainnet and contains 10,000
`starknet_getTransactionByHash` requests sampled with seed `1` from blocks
`[0, 12945536)`. It was generated against RPC v0.10.3 using snapshot
`juno_mainnet_v0.16.5_12945535.tar.zst` (SHA-256
`845c9960da60678a821a32a6c44e7b68fc6a3869743d751a0e5ff7ccf83384bd`).
The corpus and its checksum are packaged in the runner at
`/bench/rpc/corpus/v0_10/getTransactionByHash.json`.

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

## Runner image

`bench/rpc/Dockerfile` packages the scenarios and a static Go entrypoint at
`/bench/rpc/runner` into a non-root runner. It waits for RPC readiness, verifies
the Juno source commit, target chain, and snapshot head, performs an unmeasured
200-request warmup, and runs all three measured scenarios in order.
It writes `manifest.json`, `single.json`, `concurrency.json`, and
`throughput.json` to `RESULTS_DIR` (`/results` by default).
The manifest records failed checks, failed RPC and HTTP requests, VU execution
failures, and dropped and completed iterations for each scenario. Dropped
throughput iterations are recorded as saturation measurements rather than
harness failures: they indicate that the fixed worker pool could not sustain
the offered load.
At startup it removes only these known outputs from any prior run. On `TERM` or
`INT`, it stops the active command and finalizes the manifest as failed.

The runner requires `NODE_URL`, `READY_URL`, `EXPECTED_CHAIN_ID`,
`EXPECTED_BLOCK_NUMBER`, `SNAPSHOT_ID`,
`SNAPSHOT_SHA256`, `JUNO_IMAGE_DIGEST`, and `RUNNER_IMAGE_DIGEST`. The source
commit is embedded in the runner image and cannot be supplied by the
deployment. `CORPUS_PATH` defaults to the standard corpus embedded at
`/bench/rpc/corpus/v0_10/getTransactionByHash.json`; local callers may override
it for experiments. The corpus checksum must be stored next to the selected
corpus as `<CORPUS_PATH>.sha256`. `READY_TIMEOUT` defaults to `30m` and
`READY_POLL_INTERVAL` defaults to `5s`.
Any failed RPC check fails the run; the runner does not apply performance
regression thresholds.

`RUN_ID` defaults to `<UTC timestamp>-<12-character source commit>`, for example
`20260812T030000Z-afd490f8dde5`. Local callers may override it. If `RUN_ID_FILE`
is set, the runner writes the effective ID there for a deployment-side
publisher.

The runner image owns the benchmark profile. These optional variables override
its versioned defaults for local experiments or future profiles; deployments
should omit them for the standard regression suite. The effective values are
recorded in `manifest.json`.

| Optional override       | Default          | Meaning                                      |
| ----------------------- | ---------------- | -------------------------------------------- |
| `ITERATIONS`            | `200`            | requests in the sequential latency scenario |
| `VUS`                   | `50`             | workers in the fixed-concurrency scenario    |
| `CONCURRENCY_DURATION`  | `30s`            | fixed-concurrency scenario duration          |
| `THROUGHPUT_VUS`        | `50`             | fixed worker allocation for offered load     |
| `RATES`                 | `1000,2000,3000` | offered request rates per second             |
| `THROUGHPUT_DURATION`   | `5s`             | duration of each offered-load stage          |

Pull requests build and test the runner without publishing it. On pushes to
`main`, `.github/workflows/benchmark-image.yaml` publishes matched Juno and
runner images under the same immutable `sha-<commit>` tag, promotes both
successful digests to `nightly`, and reports both digests in the workflow
summary. Deployments using the moving tags must still verify the embedded
commit before measuring because two registry tags cannot be updated atomically.

The image only creates local result artifacts. Scheduling, snapshot restore,
result publication, and retention belong to the deployment configuration.

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
| `-e THROUGHPUT_VUS=<n>`         | open (`throughput.js`) | fixed worker allocation (`50`)                                |

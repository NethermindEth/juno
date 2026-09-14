# Pre-confirmed freshness benchmark (`bench/preconfirmed`)

Compare two running Juno nodes on how quickly they reflect the sequencer's
pre-confirmed block, and what each `pre_confirmed` request costs in latency.
Built for the baseline poller (500 ms ticker) versus the on-demand fetch on
RPC request (`rdr/on-demand-pre-confirmed`), but any two nodes work.

`preconfirmed_bench.py` is a standalone Python 3.9+ script (standard library
only). Every `--interval` seconds it asks the sequencer's feeder gateway for its
latest pre-confirmed block with the same delta query Juno's poller sends, then
immediately sends the same `pre_confirmed` JSON-RPC request to every node in
parallel and records the latency and the `(block_number, transaction count)`
each node returned.

## Run

### 1. Two binaries

Build the baseline from `main` in a worktree so the branch's working tree stays
untouched, then the on-demand binary from the branch:

```
git worktree add ../juno-main main
make -C ../juno-main juno && cp ../juno-main/build/juno build/juno-baseline
make juno-cached && cp build/juno build/juno-ondemand
```

`juno_version` differs between the two builds; the report prints it per node.

### 2. Two databases

Each node needs its own database directory. On APFS a copy-on-write clone is
instant and costs no extra space until the nodes diverge:

```
cp -c -R <db> <db>-b
```

### 3. Two nodes

Same flags except the binary, `--db-path`, `--http-port` and `--metrics-port`.
Pass `--gw-api-key` to both (and `--api-key` to the script) when you have one:
the nodes and the script share one IP towards the gateway.

```
build/juno-baseline --network mainnet --db-path <db>   --http --http-port 6060 --metrics --metrics-port 9090 --disable-l1-verification
build/juno-ondemand --network mainnet --db-path <db>-b --http --http-port 6061 --metrics --metrics-port 9091 --disable-l1-verification
```

Wait until both are at the tip; the script refuses to start while a node is
more than two blocks behind the sequencer.

### 4. Measure

```
python3 bench/preconfirmed/preconfirmed_bench.py \
  --node baseline=http://localhost:6060 --node ondemand=http://localhost:6061 \
  --metrics baseline=http://localhost:9090/metrics --metrics ondemand=http://localhost:9091/metrics \
  --api-key "$GW_API_KEY" --interval 0.2 --duration 300
```

Repeat with `--interval 0.1`, `0.5` and `2` back to back: on the on-demand node
every request that finds data older than 100 ms triggers a gateway fetch, so the
request rate changes both its latency and its gateway load. An iteration waits
for the slowest node, so the achieved rate (printed in the report) can be lower
than asked. Ctrl-C ends a run early and still writes the report.

Outputs land in `bench/preconfirmed/results/<timestamp>/` (git-ignored):
`samples.csv` with one row per iteration (sequencer state and, per node, send
and receive times, latency, block, tx count, error) and `report.md`, also
printed to stdout.

## Reading the report

| Column | Meaning |
| --- | --- |
| lat avg / p50 / p90 / p99 / max | request latency of the probe method, ms |
| caught up | share of iterations where the node's answer was at least the sequencer state read just before asking (same block with at least as many txs, or a newer block) |
| ahead | share where the node's answer was strictly newer than that state (it fetched between our two requests) |
| txs behind p50/p90/max | on the same block, how many txs the node was missing |
| block lag | iterations where the node's pre-confirmed block was behind the sequencer's (main-sync lag, not the poll strategy) |
| time to see p50/p90/max | from the sequencer's reply showing a new state until the client received that state (or newer) from the node; includes request latency, resolution is `--interval` |
| unseen | new sequencer states the node never returned before the run ended (the last few states of a run land here) |
| saw first | share of new states this node returned before the others (ties within 1 ms count for both) |
| gw calls / gw/req / gw non-200 | gateway `get_preconfirmed_block` calls the node made during measurement (from its `/metrics`), per probe request, and how many were not HTTP 200 |

Expected shape: the baseline answers in a few ms but its time to see spreads up
to ~500 ms plus a gateway round trip; the on-demand node answers in about a
gateway round trip with most states caught up and close to one gateway call per
request. On-demand latencies near 1 s are the wait cap in `requestChainUpdate`.

Caveats: the sequencer's tx rate varies, so compare runs taken back to back;
without an API key keep `--interval` at 0.2 s or above; the first `--warmup`
seconds (15 by default) are excluded from every number.

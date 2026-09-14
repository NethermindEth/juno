# Pre-confirmed freshness benchmark (`bench/preconfirmed`)

Compare running Juno nodes on how quickly they reflect the sequencer's
pre-confirmed block, and what each `pre_confirmed` request costs in latency and
gateway load. Built for the baseline poller (500 ms ticker) versus the on-demand
fetch on RPC request (`rdr/on-demand-pre-confirmed`), with the baseline at a
lower `--preconfirmed-poll-interval` as the obvious alternative, but any number
of nodes works.

`preconfirmed_bench.py` is a standalone Python 3.9+ script (standard library
only). It talks to the nodes only through JSON-RPC and `/metrics`, and to the
sequencer only through the public feeder gateway. Each source is sampled on its
own schedule, so no node's latency changes when another one is sampled:

- the gateway, every `--interval` seconds on a fixed grid, with the same delta
  query Juno's poller sends (`get_preconfirmed_block` with `blockIdentifier` and
  `knownTransactionCount`). A read that overruns its slot skips the missed
  slots, so the script never sends more than one gateway request per
  `--interval`;
- each node, with a `pre_confirmed` request (`--method`,
  `starknet_getBlockWithTxHashes` by default) at random, Poisson-distributed
  times with a mean gap of `--interval`. Random arrivals don't fall into step
  with a node's own timers (the on-demand node's 100 ms freshness window, a
  ticker), so each request sees the node at a random phase;
- each node's `/metrics`, at the start and the end of the measurement window.

Node requests are open loop: each one is sent at its arrival time whether or
not the node has answered the earlier ones, so the requests to a slow node
overlap instead of pushing the next one back, and every node is sampled at the
target rate whatever its latency (the report prints the rate each node got).
Waiting for each answer would give a slow node fewer requests, and time to see,
which includes the wait for the node's next request, would then charge the node
for its own latency.

- Each request in flight has its own keep-alive connection (an `http.client`
  connection carries one request at a time), from a per-node pool that reuses
  them: the pool holds as many connections as there were requests in flight at
  once. The report counts the connections each node's pool opened, more than
  that when errors closed some (a timeout closes its connection). Opening a
  connection is not counted in the latency.
- `--max-in-flight` caps the requests in flight per node. The default,
  `--timeout` / `--interval` rounded up and at least 4 (25 with the defaults),
  is how many arrive, on average, while one request runs into the timeout: only
  a node whose answers take close to `--timeout` gets near it. An arrival that
  finds the cap reached is skipped and counted (`skipped` in the report): a
  stalled node shows up there, and the script doesn't pile up threads and
  connections for it.

Every answer is recorded with its latency, block number and transaction hashes.
The hashes tell rounds apart: when a new round replaces the block at the same
height, the gateway gives it a new `block_identifier` and its transaction list
restarts. A node's answer carries no identifier, so it is placed on a round by
its transaction hashes: on the round whose list it extends or is a prefix of.

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

### 2. Three databases

Each node needs its own database directory. On APFS a copy-on-write clone is
instant and costs no extra space until the nodes diverge:

```
cp -c -R <db> <db>-b
cp -c -R <db> <db>-c
```

### 3. Three nodes

Same flags except the binary, `--db-path`, `--http-port`, `--metrics-port` and,
for the third node, a 100 ms `--preconfirmed-poll-interval` on the baseline
binary. Pass `--gw-api-key` to every node (and `--api-key` to the script) when
you have one: the nodes and the script share one IP towards the gateway.

```
build/juno-baseline --network mainnet --db-path <db>   --http --http-port 6060 --metrics --metrics-port 9090 --disable-l1-verification
build/juno-ondemand --network mainnet --db-path <db>-b --http --http-port 6061 --metrics --metrics-port 9091 --disable-l1-verification
build/juno-baseline --network mainnet --db-path <db>-c --http --http-port 6062 --metrics --metrics-port 9092 --disable-l1-verification --preconfirmed-poll-interval 100ms
```

Wait until all three are at the tip; the script refuses to start while a node's
head is more than two blocks behind the gateway's latest block (the head Juno's
poller compares itself with before it polls).

### 4. Measure

```
python3 bench/preconfirmed/preconfirmed_bench.py \
  --node baseline=http://localhost:6060 --node ondemand=http://localhost:6061 --node baseline-100ms=http://localhost:6062 \
  --metrics baseline=http://localhost:9090/metrics --metrics ondemand=http://localhost:9091/metrics --metrics baseline-100ms=http://localhost:9092/metrics \
  --api-key "$GW_API_KEY" --interval 0.2 --duration 300
```

A bare `host:port` works for both flags: `--node ondemand=localhost:6061` means
`http://localhost:6061/v0_10` and `--metrics ondemand=localhost:9091` means
`http://localhost:9091/metrics`. The script connects directly to every URL and
ignores `HTTP(S)_PROXY`/`NO_PROXY`.

Repeat with `--interval 0.1`, `0.5` and `2` back to back: the on-demand node's
latency and gateway load depend on the request rate. How it serves a
`pre_confirmed` request (`sync/preconfirmed/poller.go` on the branch):

- The request waits, for up to 1 s, until the single poll loop answers it.
  Requests are served one at a time; those that arrive during a poll wait for it
  (one in the loop's one-slot request channel, the others blocked sending to it).
- The loop answers at once if its last successful poll ended no more than
  100 ms ago. Otherwise it polls the gateway first (the latest query, plus
  backfill and class fetches when needed), answers, and resets its ticker. When
  the poll takes longer than 1 s (slow gateway, retries), the request stops
  waiting and returns whatever snapshot the node has.
- The 100 ms are counted from when a poll returned, not from when its data was
  fetched, and polls that fetched nothing count: the node was not at the tip, or
  the gateway answered 400. The clock also starts when the loop starts, so reads
  in its first 100 ms are served without a fetch. A failed poll leaves it
  unchanged, so after a failure every request polls again.
- Tick-driven polls have no freshness check, and the ticker fires whenever a
  full interval (500 ms by default) passes without an on-demand poll, so sparse
  requests don't stop it.

So its latency is bimodal (about a millisecond when served from the snapshot,
about a gateway round trip when the request polls, up to the 1 s cap when polls
are slow or failing), and its gateway responses per request fall below one as
requests get denser and rise above one when they are sparser than the ticker.
The ticker nodes poll at their interval whatever the load (back to back when a
round trip is longer than the interval).

Concurrent requests share one poll: the requests waiting on a poll are all
answered from its snapshot as soon as it ends, and the loop is fresh by then, so
none of them polls again. The script's requests overlap, so the on-demand node's
gateway responses per request fall as concurrency rises (a higher rate, slower
polls): overlapping requests add answers, not polls. That is how the node
behaves under concurrent load, not an artifact of the script.

A run ends after `--duration`, or at Ctrl-C. The script then stops sending,
reads each node's `/metrics` (the measurement window ends there) and waits up to
`--timeout` + 1 s for the answers still in flight, which count: those requests
were sent inside the window. A Ctrl-C during that wait stops it; a request still
unanswered then is abandoned, counted in `reqs` but not in errors or latency.
Either way the report is written. Outputs land in `results/<timestamp>/` next to
the script (git-ignored) unless `--out` is given: `samples.csv` with one row per
request (`source` is `sequencer` for the script's gateway reads, else the node's
name; then send and receive times, latency, HTTP status of gateway reads, block
number, tx count, hash of the last tx, the gateway's `block_identifier`, error),
and `report.md`, also printed to stdout. Rows are written as answers come in;
requests to a node overlap, so its rows are not always in send order (sort by
`t_sent_s`). An abandoned request's row has no receive time or latency, and its
error starts with `abandoned`.

## Reading the report

A new state is a block roll, a new round at the same height, or more
transactions in the same round, as the script's gateway reads show it; a state
that came and went between two reads is not counted. The first table covers
requests and gateway load, the second freshness:

| Column | Meaning |
| --- | --- |
| reqs, reqs/s | requests sent to the node during measurement, and their rate: about 1/`--interval` for every node whatever its latency (Poisson counts vary by about 1/√reqs), lower only by the skipped arrivals |
| skipped | arrivals not sent because `--max-in-flight` requests to the node were still outstanding: zero unless the node stalls or answers in about `--timeout` |
| max in flight | the most requests to the node outstanding at once, during measurement (the request itself included) |
| lat avg / p50 / p90 / p99 / max | request latency of the probe method, ms: from sending the request on an open connection until the whole answer was read. Opening a connection and decoding the JSON are not included (answers that arrive together would otherwise wait for each other's decoding) |
| gw resp, gw resp/req, gw resp/s | HTTP responses the node's feeder client got from `get_preconfirmed_block` during measurement (from `feeder_client_request_latency_count` in its `/metrics`), per probe request and per second; see "Gateway counts" |
| gw non-200 | how many of those responses were not HTTP 200; the line under the table breaks them down by status |
| compared | answers with a gateway state to compare with: the script's latest gateway read completed before the request was sent returned one (a failed or 400 read leaves the answer out). Each answer is judged against the state as of its own send time, whatever order the answers arrive in |
| caught up | share of compared answers at least as new as that state: a newer block, a newer round of the same block, or the same round with at least as many txs. That state is up to one `--interval` plus a round trip old when the request is sent, for every node alike |
| ahead | share strictly newer than that state (the node fetched after the script's read) |
| txs behind p50/p90/max | on the same block and round, how many txs the node was missing |
| block lag | answers on an older block than that state: the node had not yet fetched the new block (poll lag) or not yet stored the closed one (sync lag) |
| other round | answers on the same block but another round: one the gateway had already replaced, or one the script's reads never showed |
| time to see p50/p90/max | from the script's first gateway read showing a new state until the client received that state (or newer) from the node, ms, from whichever request brought it first: requests overlap, so a later one can come back first, and receive times decide. Includes request latency and the wait for the node's next request (mean gap `--interval`, the same for every node since requests don't wait for answers). Negative when the node returned it before the script's read did; answers received before the previous read was sent don't count, so negative values stop at the gap between two reads (one `--interval` plus a round trip, more after a timed-out read). The reference itself lags the sequencer by up to that gap, so compare nodes with each other |
| unseen | new states the node never returned before the run ended (the last few states of a run land here) |
| saw first | share of new states this node returned before every other node, each node by its earliest answer that has the state (ties within 1 ms count for each; with ties the shares add up to more than 100%) |

### Gateway counts

Juno records `feeder_client_request_latency` when an attempt gets an HTTP
response (`tryGet` in `clients/feeder/feeder.go`), so the counts are HTTP
responses, not polls or fetches:

- every attempt counts: a fetch retried after a 429 or 5xx (up to 10 retries,
  0.5-2 s apart) counts each response;
- an attempt that gets no response (a timeout, a refused or reset connection)
  counts nothing, so a node whose polls all time out shows zero; the report says
  so under the table;
- a 200 counts as a 200 even when its body then fails to decode or validate and
  the poll fails.

Backfill fetches of older pre-confirmed blocks use the same endpoint and count;
class fetches don't. No Juno metric counts polls or failed attempts, so failed
polls only show indirectly: on-demand latencies at the 1 s cap, and staleness.

Expected shape: on top of the wait for the next request, which every node has,
the baseline's time to see spreads over its 500 ms ticker plus a gateway round
trip although it answers in a few ms; the 100 ms baseline cuts that at about ten
gateway polls per second whatever the traffic; the on-demand node answers with
the bimodal latency above, has most answers caught up, and its gateway load
follows the request rate, at fewer responses per request the more its requests
overlap.

Caveats: the sequencer's tx rate varies, so compare runs taken back to back;
each request in flight holds a thread and a connection, so a stalled node costs
up to `--max-in-flight` of each: keep `--max-in-flight` times the number of
nodes well under the open-files limit (`ulimit -n`, 256 by default on macOS);
without an API key keep `--interval` at 0.2 s or above, and mind that the nodes'
own polls share the IP (about 2/s for a 500 ms ticker, about 10/s at 100 ms, up
to one per request for the on-demand node); the first `--warmup` seconds (15 by
default) are excluded from every number; Juno honours `HTTPS_PROXY` for its
gateway calls and the script doesn't, so behind a proxy the script's reads take
another path than the nodes'.

---
title: Pre-confirmed Polling
description: "How Juno fetches the pre_confirmed block, what its three polling options trade off, and which values to set for your workload."
---

# Pre-confirmed block polling

:::info
Pre-confirmed blocks are blocks that the sequencers are proposing and that will soon be finalized and added to the L2 chain. They can be queried via RPC using the `pre_confirmed` block tag.
:::

Juno unlocks the possibility for its users to decide whether to minimize response latency or data staleness (how old the data is) when they query for a pre-confirmed block by exposing these three flags:
- `--preconfirmed-poll-interval` sets how frequently Juno polls for the pre-confirmed data.
- `--preconfirmed-stale-after` sets after how much time polled data is considered old.
- `--preconfirmed-on-demand-wait` sets how long a request that triggers a poll waits for it before answering with the stored data.

In other words, `--preconfirmed-poll-interval` sets when the node updates its pre-confirmed data on its own, independently of whether there are requests for pre-confirmed data. `--preconfirmed-on-demand-wait` tells requests that are waiting for the data how long to wait for it to be refetched (worst case) before returning. Once a new pre-confirmed block has been fetched, a new one won't be refetched until `--preconfirmed-stale-after` time passes; during that window, every new request will use the cached data.

The goal with these flags is to shape how pre-confirmed polls behave when receiving a pre-confirmed request, by either answering directly with what's in memory, minimizing latency but risking answering with stale data, or, on the contrary, polling the sequencer first and waiting for the answer, maximizing data freshness at the cost of one extra round trip between Juno and the sequencer.

## Suggested presets

These flags allow users to fine-tune and decide for which main objective they want to optimize: to minimize latency, by setting `--preconfirmed-on-demand-wait` to 0, or to minimize how old the answered data is, at the cost of some latency, by being willing to let a few requests wait.

:::note
Waiting for the pre-confirmed block to be refetched from the sequencer may seem counterintuitive at first, due to the latency increase, but it returns the most up-to-date data on nearly every request. If there were no wait at all, Juno would need to be queried again to get the updated data.
:::

The following flag settings strike a good balance between latency and data staleness based on the different node use cases.

### Default (for App Developers, Stakers and Explorers)

The default preset works well for individual users who have their private Juno node and wish to minimize unnecessary requests to the sequencer, which risk getting the node rate limited for a while and effectively slowing its syncing, while at the same time striking a fine balance between fast responses and updated data.

- `--preconfirmed-poll-interval 1s`
- `--preconfirmed-stale-after 250ms`
- `--preconfirmed-on-demand-wait 300ms`

### RPC Providers

RPC provider nodes are expected to be constantly hit with pre-confirmed requests, and because nodes are shared with a lot of users, triggering a request on demand but not waiting for it is OK, because data staleness should be small, and the next requests will benefit from this on-demand poll.

- `--preconfirmed-poll-interval 500ms`
- `--preconfirmed-stale-after 250ms`
- `--preconfirmed-on-demand-wait 0s`

### Bots and MEV

Users looking to maximize data freshness at the cost of some initial latency will benefit from waiting longer for the poll, to get the most recent data on nearly every read.

- `--preconfirmed-poll-interval 500ms`
- `--preconfirmed-stale-after 250ms`
- `--preconfirmed-on-demand-wait 1s`

## Metrics

The [metrics endpoint](monitoring), enabled with `--metrics` and served on `localhost:9090` by default, shows how the options behave on your node under your own traffic. To find the values that work best for you, change one option at a time, let the node serve its usual traffic for a few minutes, and compare the numbers below before and after.

### Polls

Juno reports every pre-confirmed poll it sends while it is at the tip of the chain, whether `--preconfirmed-poll-interval` or a read triggered it. Grep for the metrics will show you the full list:

```bash
curl -s http://localhost:9090/metrics | grep ^sync_preconfirmed | grep -v _bucket
```

Result example after running the previous command:

```
sync_preconfirmed_backfill_gap_sum 612
sync_preconfirmed_backfill_gap_count 604
sync_preconfirmed_failed_polls{reason="error"} 0
sync_preconfirmed_failed_polls{reason="not_found"} 3
sync_preconfirmed_failed_polls{reason="rate_limited"} 0
sync_preconfirmed_poll_latency_sum{update="delta"} 187.28
sync_preconfirmed_poll_latency_count{update="delta"} 2341
sync_preconfirmed_poll_latency_sum{update="full"} 96.64
sync_preconfirmed_poll_latency_count{update="full"} 604
sync_preconfirmed_poll_latency_sum{update="no_change"} 63.5
sync_preconfirmed_poll_latency_count{update="no_change"} 1270
```

- `sync_preconfirmed_poll_latency_count` counts the successful polls since the node started, by what they found: `full` a block the node did not have yet, `delta` new transactions in the block it had, and `no_change` nothing new. Run the command twice, a minute apart, and divide the difference by 60 to get the polls per second. 
- The share of `no_change` polls shows whether the node polls faster than the block changes. A high share means most polls find nothing new, and a longer `--preconfirmed-poll-interval` or `--preconfirmed-stale-after` sends fewer requests to the gateway. A low share means the block changes between most polls, and shorter values would answer with fresher data, at the cost of more requests.
- `_sum` divided by `_count` is the average duration of a poll in seconds: 0.08 s for `delta` polls in the example. 
- `sync_preconfirmed_failed_polls` counts the failed polls by reason: `not_found` when the gateway had no pre-confirmed block to return, which is harmless; `rate_limited` when the gateway refused the poll (see [Throttling](#throttling)); and `error` for any other failure, which the node also logs as a warning.
- `sync_preconfirmed_backfill_gap` is recorded each time the gateway has moved on to a new block, with how many blocks below it the node fetches: the block it had, to complete it, plus any block it never saw. A node that keeps up fetches 1 each time, so `_sum` stays close to `_count`. 
### Read latency

Juno records the latency of every RPC method. For `starknet_getBlockWithTxHashes`, for example:

```bash
curl -s http://localhost:9090/metrics | grep rpc_server_requests_latency | grep starknet_getBlockWithTxHashes | grep -v _bucket
```

`_sum` divided by `_count` is the average latency in seconds. It covers reads of every block, not only `pre_confirmed`, so compare it before and after a change rather than reading it on its own.

### Throttling

The gateway limits requests per IP address and refuses the excess with HTTP 429. The node's block synchronization counts against the same limit, and so does every other node or client behind the same address. The refused polls are counted by

```
sync_preconfirmed_failed_polls{reason="rate_limited"}
```

which should stay at zero at your production poll rate. As Prometheus queries, the polls per second by outcome:

```
sum by (update) (rate(sync_preconfirmed_poll_latency_count[5m]))
sum by (reason) (rate(sync_preconfirmed_failed_polls[5m]))
```

If the refused polls grow, lengthen `--preconfirmed-stale-after`, which sets the poll rate under load, or give the node its own egress IP before lowering any of the three values: a refused poll makes the next read poll again, so a throttled node polls more and answers staler, although its reads never fail. 

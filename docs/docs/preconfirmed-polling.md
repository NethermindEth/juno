---
title: Full Control Over Pre-confirmed Polling
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

The goal with these flags is to shape how pre-confirmed polls behave when receiving a pre-confirmed request, by either answering directly with what's in memory, minimizing latency but risking answering with stale data, or, on the contrary, polling the sequencer first and waiting for the answer, maximizing data freshness at the cost of one extra round trip between Juno and the sequencer.

## Suggested presets

Depending on the node's use case, the following flag settings strike a good balance between latency and data staleness.

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

## Study on the effect of these flags

:::info
The following optional section shows how each of these options affects the node's behaviour and how the default values above were chosen, for the curious and for those looking to fine-tune the flags to their own conditions and use case.
:::

The four figures below play the same two seconds: the gateway's pre-confirmed state changes three times (A to B, C and D), every poll takes 120 ms, and a few `pre_confirmed` reads arrive at the same moments. For each read the figure shows whether it was answered with the gateway's current state (filled dot) or an older one (hollow dot), how long it waited, and the age of the data it received.

### Tick only

![Timeline of the tick-only configuration: the node polls every 500 ms, reads never wait, and the data a read gets is between 120 and 620 ms old](/img/preconfirmed/timeline-tick-only.svg)

`500ms / 500ms / 0s` is the configuration Juno shipped before the current defaults. The node polls every 500 ms and reads never wait. Because stale-after equals the tick, the stored data always counts as fresh and a read never triggers a poll. A reader gets data between one round trip and the tick plus one round trip old, so a read that lands just before a tick completes can miss a state the gateway published a few hundred milliseconds earlier. The gateway cost is a flat 2.5 requests per second whether or not anybody reads.

### On-demand polling without waiting

![Timeline of on-demand polling without a wait: a read older than 250 ms triggers a poll but answers with the old data, and a burst of four reads shares one poll](/img/preconfirmed/timeline-on-demand-no-wait.svg)

With `1s / 250ms / 0s` the tick is twice as slow, so a node that nobody reads sends 1.4 to 1.6 gateway requests per second instead of 2.5, about 40% fewer. A read that finds the stored data older than 250 ms triggers a poll but answers immediately with the old data; only a read that arrives after that poll completes benefits. A burst of reads shares a single triggered poll, and with no wait every read in the burst gets the old data. For a sparse reader this is strictly worse than the 500 ms tick: measured, only 40% of such reads were caught up (p90 staleness 700 ms), and bursts fared no better (53%).

### On-demand polling with a wait

![Timeline of on-demand polling with a 300 ms wait: the triggering read and the burst wait one round trip and answer with the gateway's current state; reads inside the fresh window answer at once](/img/preconfirmed/timeline-on-demand-wait.svg)

With `1s / 250ms / 300ms`, the current defaults, the triggering read waits for its poll (one round trip, capped at 300 ms) and answers with the gateway's current state; a burst waits together and every read in it is caught up. Reads arriving within 250 ms of a completed poll answer at once. The tick is still 1 s, so the idle cost stays 40% below the 500 ms tick. Measured, sparse reads were caught up 100% of the time at a median latency of 121 ms; at 20 reads per second half of the reads still answered in under a millisecond (p50 0.8 ms), about a quarter waited more than 100 ms (p99 242 ms), and 95% were caught up.

### The 500 ms tick with the wait

![Timeline of the 500 ms tick with a 300 ms wait: about half of the sparse reads find data younger than 250 ms and answer at once, the rest wait one round trip](/img/preconfirmed/timeline-500-tick-wait.svg)

`500ms / 250ms / 300ms` keeps the wait but ticks twice as often, so about half of the sparse reads find data younger than 250 ms and answer immediately (median latency 0.7 ms instead of 121 ms). The price is the idle gateway cost of the previous default, 2.4 requests per second. Under load the two ticks behave identically, because polling is read-driven and the tick never fires.

## What the measurements showed

The numbers on this page were measured on mainnet over a 2-hour run on 2026-10-07: one node was restarted with each of twelve configurations and, for each one, served `pre_confirmed` reads at 0.2, 5 and 20 requests per second and in bursts of ten, while a second process read the gateway directly so that every answer could be compared with what the gateway had at that moment.

![Scatter plot of gateway requests per second against the share of caught-up reads for every configuration, for a sparse reader and for a node serving 20 reads per second](/img/preconfirmed/cost-vs-freshness.svg)

Two things stand out:

- **No tick and stale-after pair with a wait of `0s` beats the previous default.** A longer tick is cheaper when idle, but for a sparse reader every read then gets data up to a tick old, and polling on demand without waiting only helps the *next* read. Under load the tick does not matter at all: polls are read-driven, one every round trip plus stale-after, so stale-after alone sets both cost and freshness, and the previous default already sits on that curve.
- **The wait is what makes the longer tick acceptable.** The 1 s tick costs 1.4 to 1.6 gateway requests per second when idle instead of 2.5. With the 300 ms wait, the reads that would have been stale wait one round trip and come back caught up, so the 1 s tick with the wait is fresher than the previous default at every load level, and cheaper at every level but the busiest, where its read-driven polls make about 10% more requests (2.8 against 2.5 per second at 20 reads per second).

![Bar charts of caught-up share and read latency p99 for waits of 0, 150, 300 and 1000 ms at four load levels](/img/preconfirmed/wait-sweep.svg)

**Why 300 ms.** A poll takes 150 to 250 ms end to end on the 1 s tick (a poll that finds the gateway on a new block also re-polls the block it had, a second round trip). A 150 ms cap times out on about one poll in five, and those reads get the old data after waiting for nothing. A 1 s cap gains 2 to 13 points of freshness depending on the load level, at the edge of the run's noise, and lengthens the tail: the slowest reads take up to 500 ms instead of 300.

**Why 250 ms.** Stale-after sets the poll rate under load. With `100ms` or `0s` the node sent 3.8 and 5.3 gateway requests per second at 20 reads per second, enough for the gateway to refuse 12% and 28% of its polls, and because a refused poll does not advance the freshness clock, the next read polls again. `250ms` keeps a busy node at about 2.8 requests per second with 95% of reads caught up; `500ms` is cheaper (2.0 per second) but leaves 40% of reads under load stale.

| | previous default `500ms / 500ms / 0s` | **default `1s / 250ms / 300ms`** | runner-up `500ms / 250ms / 300ms` |
| --- | --- | --- | --- |
| gateway requests/s, idle (no reads) | 2.5 | **1.4** | 2.4 |
| gateway requests/s, 20 reads/s | 2.5 | **2.8** | 2.8 |
| caught up: sparse reads / 20 reads/s / bursts | 76% / 79% / 75% | **100% / 95% / 85%** | 92% / 95% / 97% |
| staleness p90: sparse / 20 reads/s / bursts | 200 / 140 / 130 ms | **0 / 0 / 100 ms** | 0 / 0 / 0 ms |
| read latency p50: sparse / 20 reads/s / bursts | 0.7 / 0.7 / 1.2 ms | **121 / 0.8 / 119 ms** | 0.7 / 0.8 / 66 ms |
| read latency p99, worst load level | 3.5 ms | **300 ms** | 300 ms |

Rounded; "idle" is a node with no reads at all, "sparse" is one read every five seconds, "bursts" are ten simultaneous reads every two seconds.

## Why the default is 1s / 250ms / 300ms

Juno's defaults are `preconfirmed-poll-interval 1s`, `preconfirmed-stale-after 250ms` and `preconfirmed-on-demand-wait 300ms`. When nobody is reading, the node polls the feeder gateway once a second, about 40% fewer requests than the 500 ms tick it used before. A `pre_confirmed` read that finds the stored block older than 250 ms triggers a poll (or joins the one in flight) and waits up to 300 ms for it, so it answers with the gateway's current state instead of a copy up to a second old. Measured on mainnet, this answered 85% to 100% of reads with the gateway's current state at every load level, against 75% to 79% for the previous default, for a latency of about one gateway round trip (p50 120 ms) on the reads that have to wait; at 20 reads per second half of the reads still answer in under a millisecond and three in four within 100 ms. Stale-after 250 ms keeps a busy node near 2.8 gateway requests per second, below the rate at which the gateway starts refusing requests, and 300 ms is the shortest wait that covers nearly every poll.

## Presets by use case

The defaults suit most nodes. Two kinds of operator trade the idle saving for something they value more.

### RPC providers

```
--preconfirmed-poll-interval 500ms --preconfirmed-stale-after 250ms --preconfirmed-on-demand-wait 150ms
```

A provider's node is never idle, so the 1 s tick saves nothing, and in a quiet moment the tick is what sets how soon pre-confirmed subscriptions advance and how many sparse reads have to wait: use `500ms`. The wait is the latency knob. `150ms` caps every `pre_confirmed` read at about 152 ms while still answering 87% of reads at 20 per second with the gateway's current state (95% with `300ms`); about three reads in ten wait more than 100 ms. For a strict sub-millisecond latency target on this method, use `--preconfirmed-on-demand-wait 0s`: reads answer in about a millisecond at the freshness of the previous default (78% caught up at 20 per second). Under load the node makes about 2.7 gateway requests per second at 20 reads per second and at most about 3 at any higher rate, so the gateway cost scales with the number of nodes, not with traffic; give each node its own egress IP, since two nodes behind one address exceed the rate at which the gateway starts refusing requests.

### Block explorers

The defaults. Explorer traffic is page loads: bursts of a few reads with seconds between them. With any configuration that does not wait, a burst arriving two seconds after the previous one shares one poll and every read in it gets the old data (53% caught up at the 1 s tick, 69% to 77% at 500 ms). With the 300 ms wait the same bursts answer 85% caught up for a median of about 120 ms per read, which no page load notices. If the pre-confirmed transaction list should be complete rather than mostly complete, `--preconfirmed-on-demand-wait 1s` lifts bursts to 98% at the same gateway cost, with a latency tail up to half a second. An explorer that streams pending transactions over WebSocket should set the tick to `500ms`, since subscriptions on a node with no readers advance at the tick.

### Staking validators

The defaults. The attestation path of the staking tool reads `latest` block headers and closed blocks, which these options do not touch. The one call that reaches the pre-confirmed poller is the transaction status check of the attest transaction, which costs one bounded round trip (p99 about 250 ms) against a window of a minute or more. Removing the wait would not add stability, since the wait is already bounded and released as soon as a poll fails, and it would make the status check see `PRE_CONFIRMED` one poll later. A node that serves only the validator makes about 1.5 pre-confirmed requests per second, about 3 per second with the synchronizer's own calls, half the rate at which the gateway starts refusing requests.

### Freshness first

```
--preconfirmed-poll-interval 500ms --preconfirmed-stale-after 250ms --preconfirmed-on-demand-wait 1s
```

For searchers and bots that poll densely or in bursts and want the gateway's current state on every read. Stale-after `250ms` with a wait is the ceiling of what these options deliver: 95% to 98% of reads caught up with a p90 staleness of 0 ms at 5 and 20 reads per second and in bursts. The `1s` cap never gives up on a slow poll (a poll that fetches a new block takes about 240 ms), at the price of a latency tail to half a second. The `500ms` tick halves the age that subscriptions see between bursts and halves the share of sparse reads that have to wait. Do not set stale-after to `0s` or `100ms` without a throttling bypass you have verified: at 20 reads per second, `0s` made 5.3 requests per second, had 28% of them refused and was no fresher than `250ms` with a wait, because a refused poll makes the next read poll again.

## Check that the gateway is not throttling your node

Every preset above keeps a single node below the rate at which the gateway started refusing requests from the test machine, about six pre-confirmed requests per second from one IP address (the node's polls plus the measurement's own reads). The node's other sync calls, about 1.5 per second, count against the same budget, and so does every other node or client behind the same egress IP. Verify on your node with the [metrics endpoint](monitoring): the refused polls are counted by

```
feeder_client_request_latency_count{method="/feeder_gateway/get_preconfirmed_block",status="429"}
```

which should stay at zero at your production poll rate, and the same counter with `status="200"` gives the poll rate itself. As a Prometheus query:

```
sum by (status) (rate(feeder_client_request_latency_count{method="/feeder_gateway/get_preconfirmed_block"}[5m]))
```

If the 429 count grows, lengthen `preconfirmed-stale-after` or give the node its own egress IP before lowering any of the three values. An API key passed with `--gw-api-key` is not evidence of headroom on its own: the key used in the measurements did not lift the per-IP limit.

---
title: Full Control Over Pre-confirmed Polling 
description: "How Juno fetches the pre_confirmed block, what its three polling options trade off, and which values to set for your workload."
---

# Pre-confirmed block polling

:::info
Pre-confirmed blocks are blocks that the Sequencers are proposing and that will soon be finalized and added to the L2 chain. They can be queried via RPC using the `pre_confirmed` block tag.
:::

Juno unlocks the possibility to its users to decide to minimze response latency or data staleness (how old the data is) when they query for a pre-confirmed block by exposing these three flags:
- `--preconfirmed-poll-interval` sets how frequently Juno polls for the pre-confirmed data.
- `--preconfirmed-stale-after` sets after how many time polled data is considered old.
- `--preconfirmed-on-demand-wait` sets how long should requests for pre-confirmed data wait for a response.

The goal with these flags is to shape how pre-confirmed polls behave by either when receiving a preconfirmed request answer directly with what's on memory, minimizing latency but risking answering stale data, or on the contrary, forwarding the request to the sequencer, maximizing data freshness at the cost of one extra roundtrip between Juno and the sequencer.

## Suggested presets 

Depending on the node use case, the following are flag settings the team recommend that find a good balance between latency and data staleness. 

### Default (for App Developers, Stakers and Explorers )

The default preset works well for indidivual users who have their private Juno node and wish to minimize unnecessary requests to the Sequencer and risk getting rate limited for a while, effectively slowing the node syncing while at the same time striking a fine balance between fast responses and updated data.

- `--preconfirmed-poll-interval 1s`
- `--preconfirmed-stale-after 250ms`
- `--preconfirmed-on-demand-wait 300ms`

### RPC Providers

RPC providers nodes are expected to be constantly hit with pre-confirmed requests and because nodes
are shared with a lot of users, triggering a request on demand but not waiting for it is ok, because data staleness should be small, and next requests will benefit from this on-demand.

- `--preconfirmed-poll-interval 500ms`
- `--preconfirmed-stale-after 250ms`
- `--preconfirmed-on-demand-wait 0s`

### Bots and MEV

For users looking to maximize freshness data at the cost of some initial latency, they will benefit from waiting longer periods of time, to guarantee always getting the most recent data.

- `--preconfirmed-poll-interval 500ms`
- `--preconfirmed-stale-after 250ms`
- `--preconfirmed-on-demand-wait 1s`

## Study on the effect of this flags 

:::info
The following is an optional section to explain how the previous default values were achieved and how they actually impact the node behaviour. For the curious and for the ones looking to fine-tune this flags to their unique conditions and use case.
:::

### The setup

Everything below was measured on one Juno node on mainnet on 2026-10-07, restarted with each configuration in turn and serving `pre_confirmed` reads (`starknet_getBlockWithTxHashes`) under five read patterns: none, one read every five seconds ("sparse reads"), five per second, twenty per second, and bursts of ten at once about every two seconds. Juno fetches pre-confirmed blocks from the sequencer's feeder gateway, called the gateway below, and a second process read the gateway directly throughout, so that every answer the node gave could be compared with the gateway's state at that instant.

Four numbers describe each configuration:

- **Gateway requests per second**: how often the node asks the gateway for the pre-confirmed block; one such request is a poll, a round trip of about 120 ms. This is what a configuration costs the gateway, which limits requests per IP address.
- **Caught up**: the share of reads answered with the gateway's state at that instant; a read that is not caught up is one state behind (the gateway already had more transactions, or a newer block).
- **Staleness**: for a read that is not caught up, how long the gateway had already had a newer state, as a 90th percentile (p90); 0 ms means at least nine reads in ten were caught up.
- **Read latency**: how long the `pre_confirmed` RPC read takes, as a median (p50) and a 99th percentile (p99).

Differences of up to about five caught-up points or 100 ms of staleness are run-to-run noise; the sparse-read figures rest on 20 to 30 reads each.

Three terms name what the options control, and a configuration is written as `interval / stale-after / wait`. The **tick** is the timer: left alone, the node polls once per `--preconfirmed-poll-interval`. The **fresh window** is `--preconfirmed-stale-after`: a read that arrives within that time after the last successful poll answers at once from the stored block, and a later read triggers a new poll or joins the one in flight (a poll that finds no pre-confirmed block at the gateway still counts as successful; a refused or failed one does not). The **wait** is `--preconfirmed-on-demand-wait`: how long such a read waits for that poll before answering with whatever is stored.

Each configuration below opens with a schematic timeline that applies these rules to the same two seconds: the same gateway state changes (A to B, C and D), the same 120 ms polls and the same eight reads, four of them at once at 1300 ms. A filled dot is a read that was caught up, a hollow dot one state behind; the shaded band after each completed poll is the fresh window; the bottom lane is the age of the data each read gets, counted from when its poll was sent.

### Polling on a timer only: `500ms / 500ms / 0s`

![Timeline of polling on a timer only: polls every 500 ms, no read waits, three of eight reads one state behind](/img/preconfirmed/timeline-tick-only.svg)

Your node polls the gateway every 500 ms, whatever anyone reads. The fresh window equals the tick, so the stored block always counts as fresh and a read never triggers a poll; with no wait it never waits either. A read gets what the last poll fetched, data between one round trip (120 ms) and a tick plus a round trip (620 ms) old, so whether it is caught up is a matter of timing: in the timeline the read at 600 ms gets A although the gateway moved to B at 350 ms. The gateway's state changes once or twice a second on mainnet (a new block about every two seconds, plus the transactions added in between), so about one read in four is one state behind.

- **What you get:** every read answers in about a millisecond (p50 0.7 ms, p99 about 1 ms, 3.5 ms in bursts); no read ever waits.
- **What it costs:** a flat 2.5 gateway requests per second whether or not anybody reads, and about one read in four one state behind whatever the read pattern (75% to 79% caught up, staleness p90 about 130 to 200 ms).

### Polling on demand without waiting: `1s / 250ms / 0s`

![Timeline of polling on demand without a wait: a stale read triggers a poll but answers with the old data, and a burst shares one poll](/img/preconfirmed/timeline-on-demand-no-wait.svg)

The tick is 1 s, so a node nobody reads polls once a second. The fresh window is 250 ms: a read that finds the stored block older than that triggers a poll but, with no wait, answers at once with the old data, and the poll proceeds for whoever reads next. A poll therefore helps the reads that follow it, never the read that triggered it: in the timeline the read at 600 ms answers with A while the gateway is already on B, and the burst at 1300 ms shares one poll and all four reads answer B while the gateway is on C. That suits a node read continuously and does nothing for a sparse reader. A successful read-triggered poll restarts the tick, so under load the tick never fires (the hollow triangles are the ticks cancelled) and polls run back to back, one every round trip plus 250 ms plus the gap to the next read: the node is then about as fresh as with the timer alone.

- **What you get:** as few gateway requests as a 1 s tick allows when nobody reads (1.6 per second, about a third fewer than the 500 ms tick) and reads that still never wait (p99 about 1 ms, 4 ms in bursts).
- **What it costs:** for anyone who reads rarely or in bursts, data up to a second old: 40% of sparse reads caught up with a staleness p90 of 700 ms, and 53% of burst reads, since a burst shares one poll that none of its reads waits for. Under load, nothing gained: 78% caught up at 20 reads per second for 2.8 requests per second.

### Polling on demand and waiting for the result: `1s / 250ms / 300ms` (the default)

![Timeline of polling on demand with a 300 ms wait: reads that trigger or join a poll wait one round trip and are caught up, reads inside the fresh window answer at once](/img/preconfirmed/timeline-on-demand-wait.svg)

Same tick and fresh window; the read that triggers or joins a poll waits for it, at most 300 ms. A poll is one round trip, so such a read answers about 120 ms later with the state the gateway had at that moment, and reads inside the fresh window still answer at once. A waiting read is released the instant its poll completes or fails, so the cap only bites when a poll is slow. The polls in the timeline are the same as without the wait; what changes is who waits for them: the read at 600 ms waits 120 ms and gets B, the burst waits together for one poll and gets C, and every read is caught up at no extra gateway cost.

- **What you get:** the fewest gateway requests when nobody reads (1.4 per second) and 85% to 100% of reads caught up whatever the read pattern: every sparse read, 95% at 20 reads per second, 85% in bursts, with a staleness p90 of 0 ms for sparse reads and at 20 reads per second.
- **What it costs:** the reads that trigger or join a poll wait for it: three sparse reads in four and every read in a burst (p50 about 120 ms), and at 20 reads per second about one read in four waits more than 100 ms although the p50 stays at 0.8 ms. The p99 is the cap, about 300 ms, whatever the read pattern. At 20 reads per second the node also polls about 10% more often than the timer alone (2.8 against 2.5 requests per second), because polls then run every round trip plus 250 ms.

### A faster timer with the wait: `500ms / 250ms / 300ms`

![Timeline of the 500 ms tick with a 300 ms wait: more reads answer at once from inside a fresh window, and one of them is still one state behind](/img/preconfirmed/timeline-500-tick-wait.svg)

The fresh window and the wait are the default's; the tick is 500 ms. With a poll every 500 ms and a fresh window of 250 ms, a sparse read has about an even chance of landing inside a window and answering at once; the other half trigger or join a poll and wait. On a node nobody reads, subscriptions advance every 500 ms instead of every second. In the timeline the read at 600 ms joins the tick's poll in flight and waits only 20 ms, and the read at 1750 ms answers at once from inside a fresh window but is one state behind, because the gateway moved to D just after that poll was sent: the fresh window bounds the age of the data, not whether the gateway has changed since.

- **What you get:** the default's freshness with fewer reads waiting: about half of the sparse reads answer at once (p50 0.7 ms instead of 121 ms), bursts wait less (p50 66 ms instead of 119 ms), and subscriptions advance twice as often on a quiet node. At 20 reads per second the two configurations measured the same; in bursts and at 5 reads per second this one measured about ten caught-up points more, at the edge of the noise band.
- **What it costs:** 2.4 gateway requests per second when nobody reads, one more per second than the default, and sparse reads caught up 92% of the time rather than every time.

### What each option changes

In the figure below, up and to the left is better: fewer gateway requests, more reads caught up.

![Gateway requests per second against caught-up share for every configuration measured, for a sparse reader and at 20 reads per second](/img/preconfirmed/cost-vs-freshness.svg)

**The poll interval sets the idle cost and how soon a quiet node moves.** With no reads, the tick alone sets the gateway cost: about 2.5 requests per second at 500 ms, 1.4 to 1.6 at 1 s, 1.1 at 2 s (one poll per tick, plus about one extra request per new block). It also sets how soon WebSocket subscriptions advance on a node nobody reads (every poll that fetches something new reaches them, whether the tick or a read triggered it) and, with no wait, how old a sparse read's data can be: in the upper panel the configurations without a wait fall from 76% caught up at 500 ms to 40% at 1 s and 4% at 2 s. Under load every poll is read-triggered and the interval stops mattering: in the lower panel `1s / 250ms / 0s` and `2s / 250ms / 0s` sit on the same spot, and so do `1s / 250ms / 300ms` and `500ms / 250ms / 300ms`.

**Stale-after sets the poll rate under load, and with it whether the gateway throttles your node.** While reads keep coming, the node polls once every round trip plus stale-after plus the gap to the next read. At 20 reads per second, `500ms` gave 2.0 requests per second with 40% of reads one state behind; `250ms` about 2.8 with 95% caught up (78% without the wait); `100ms` 3.8, of which the gateway refused 12% (HTTP 429); `0s` 5.3 with 28% refused. A refused poll does not advance the fresh window's clock, so the next read polls again: throttling makes a node poll more, not less, while its reads fall back to the stored block. Neither `100ms` nor `0s` was fresher than `250ms` with the wait, and with no reads stale-after changes nothing.

**The wait sets what the triggering read gets, not the gateway cost.** On the 1 s tick with the 250 ms fresh window, waits of 0, 150, 300 and 1000 ms cost the same (1.5 to 1.6 requests per second with sparse reads, 2.7 to 2.8 at 20 reads per second): the reads trigger the same polls whether or not they wait for them. What moves is the share of reads caught up and the latency tail, which the cap sets directly.

![Caught-up share and read latency p99 for waits of 0, 150, 300 and 1000 ms, per read pattern](/img/preconfirmed/wait-sweep.svg)

A poll normally takes one round trip, but a poll that finds the gateway on a new block takes a second round trip to complete the block it was following, about 240 ms in all. A 150 ms cap therefore timed out on about one poll in five, and the read then answers with the old data after waiting for nothing: 81% of sparse reads caught up against 100% with 300 ms. A 300 ms cap covers nearly every poll. A 1 s cap never gives up on a slow poll; it lifts bursts and 5 reads per second by about ten points, a gain near the noise band, and stretches the slowest reads to half a second.

### Putting it together

The table compares the presets with polling on a timer only. `500ms / 250ms / 0s`, the RPC Providers preset, behaves like `500ms / 500ms / 0s` under load, where stale-after sets the poll rate, and should for sparse reads too: its measured 50% rests on 22 reads taken while the gateway's state was changing faster than in any other sparse-read measurement, the likely reason for its lower figure.

| | `500ms / 500ms / 0s` | `1s / 250ms / 300ms` (default) | `500ms / 250ms / 300ms` | `500ms / 250ms / 0s` |
| --- | --- | --- | --- | --- |
| gateway requests/s: no reads / 20 reads/s | 2.5 / 2.5 | 1.4 / 2.8 | 2.4 / 2.8 | 2.5 / 2.5 |
| caught up: sparse / 20 reads/s / bursts | about 76% / 79% / 75% | 100% / 95% / 85% | 92% / 95% / 97% | 50% / 78% / 69% |
| staleness p90: sparse / 20 reads/s / bursts | about 200 / 140 / 130 ms | 0 / 0 / 99 ms | 0 / 0 / 0 ms | about 320 / 150 / 250 ms |
| read latency p50: sparse / 20 reads/s / bursts | 0.7 / 0.7 / 1.2 ms | 121 / 0.8 / 119 ms | 0.7 / 0.8 / 66 ms | 0.7 / 0.7 / 1.2 ms |
| read latency p99, worst read pattern | 3.5 ms | 302 ms | 301 ms | 3.4 ms |

**The default preset.** Your node polls the gateway least when nobody reads, answers nearly every read with the gateway's current state, and stays well under the rate at which the gateway starts refusing requests. What it gives up is one round trip of latency on the reads that trigger a poll, with the p99 at the cap, bursts caught up 85% of the time rather than always, and subscriptions that advance only once a second on a node nobody reads.

**The RPC Providers preset.** Measured directly: every read answers in about a millisecond, the gateway cost is a flat 2.5 requests per second with no refused polls, and subscriptions advance at least every 500 ms; a 1 s tick would save nothing on a node that is never idle. What it gives up is the triggering read's freshness: under load about one read in five is one state behind, in bursts about one in three, since a burst shares one poll that no read in it waits for.

**The Bots and MEV preset.** `500ms / 250ms / 1s` itself was not measured; what follows is inferred from `1s / 250ms / 1s`, which shows what the 1 s wait gives, and from `500ms / 250ms / 300ms` against the default, which shows what the 500 ms tick changes. The 1 s wait gave 96% to 98% of reads caught up whatever the read pattern, not every read, for a latency tail to half a second at the same gateway cost as the 300 ms wait. The 500 ms tick changes nothing under load, lets about half of the sparse reads answer at once, and costs 2.4 requests per second when nobody reads. The combination should therefore give the freshness of the 1 s wait with the latency profile of the 500 ms tick. Stale-after stays at `250ms` because `100ms` and `0s` bought no freshness, only refused polls.

## Check that the gateway is not throttling your node

The gateway limits requests per IP address. In the measurements, refusals (HTTP 429) began at about six pre-confirmed requests per second from one address, counting the node's polls and the measurement's own reads of the gateway together, and became frequent from seven. The node's block synchronization makes about 1.5 other gateway calls per second that count against the same budget, and so does every other node or client behind the same address. Every preset above keeps a single node under that limit: at most about 2.8 pre-confirmed requests per second at 20 reads per second and, since polls cannot run faster than one per round trip plus stale-after, at most about 3 at any higher rate. Two nodes behind one address do not.

Verify on your node with the [metrics endpoint](monitoring). The refused polls are counted by

```
feeder_client_request_latency_count{method="/feeder_gateway/get_preconfirmed_block",status="429"}
```

which should stay at zero at your production poll rate; the same counter with `status="200"` counts the successful requests. As a Prometheus query:

```
sum by (status) (rate(feeder_client_request_latency_count{method="/feeder_gateway/get_preconfirmed_block"}[5m]))
```

If the 429 count grows, lengthen `--preconfirmed-stale-after`, which sets the poll rate under load, or give the node its own egress IP before lowering any of the three values: a refused poll makes the next read poll again, so a throttled node polls more and answers staler, although its reads never fail. An API key passed with `--gw-api-key` is not evidence of headroom on its own: the key used in the measurements did not lift the per-IP limit.

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

Everything below was measured on one Juno node on mainnet on 2026-10-07. The node was restarted and re-synced with each configuration of the three options in turn, then served `pre_confirmed` reads (`starknet_getBlockWithTxHashes` with the `pre_confirmed` tag) under the same five read patterns: no reads at all, one read every five seconds ("sparse reads" below), five reads per second, twenty reads per second, and bursts of ten simultaneous reads about every two seconds. The sequencer's feeder gateway, the Sequencer of the sections above, is called the gateway from here on. A second process read it directly throughout, so that every answer the node gave could be compared with the state the gateway had at that instant.

Four numbers describe each configuration:

- **Gateway requests per second**: how often the node asks the gateway for the pre-confirmed block. A poll is one such request, a gateway round trip of about 120 ms from the test machine. This is what a configuration costs, and what the gateway's per-IP throttling counts.
- **Caught up**: the share of reads whose answer was still the gateway's state at that instant. A read that is not caught up is one state behind: the gateway already had more transactions, or a newer block.
- **Staleness**: for a read that is not caught up, how long the gateway had already had a newer state, given as a 90th percentile (p90). 0 ms means at least nine reads in ten were caught up.
- **Read latency**: how long the `pre_confirmed` RPC read takes, from sending the request to receiving the whole answer, as a median (p50) and a 99th percentile (p99).

Two runs of the same configuration two hours apart differed by up to five caught-up points and about 100 ms of staleness p90; smaller differences are noise. The sparse-read numbers rest on 20 to 30 reads each and move in steps of several points.

Three terms name what the options control. The **tick** is the timer: left alone, the node polls once per `--preconfirmed-poll-interval`. The **fresh window** is `--preconfirmed-stale-after`: a read that arrives within that time after the last successful poll answers at once from the stored block; a later read triggers a new poll, or joins the one in flight. The **wait** is `--preconfirmed-on-demand-wait`: how long such a read waits for that poll before answering with whatever is stored. The read is released as soon as the poll completes, or at once if the poll fails; with a wait of `0s` it answers immediately from the stored block while the poll proceeds for later reads. A poll that finds no pre-confirmed block at the gateway still counts as successful for the fresh window; a refused or failed poll does not. A successful read-triggered poll restarts the tick, and every poll that fetches something new is published to WebSocket subscriptions, whether the tick or a read triggered it.

Each configuration below opens with a timeline. All four play the same two seconds: the gateway's state changes at the same moments (A to B at 350 ms, to C at 900 ms, to D at 1550 ms), every poll takes 120 ms, and the same reads arrive at the same moments: at 600 ms, at 800 ms, four at once at 1300 ms, at 1750 ms and at 1950 ms. A filled dot is a read that was caught up, a hollow dot one state behind. The shaded band after each completed poll is the fresh window. The bottom lane is the age of the data each read gets, counted from the moment its poll was sent.

### Polling on a timer only: `500ms / 500ms / 0s`

![Timeline of polling on a timer only: the node polls every 500 ms, reads never wait, and three of the five read moments answer one state behind](/img/preconfirmed/timeline-tick-only.svg)

Your node polls the gateway every 500 ms, whatever anyone reads. The fresh window equals the tick, so the stored block always counts as fresh and a read never triggers a poll; with no wait it never waits either. A read gets what the last poll fetched: data between one round trip (120 ms) and a tick plus a round trip (620 ms) old, depending on where it falls between two polls.

In the timeline, the read at 800 ms and the burst at 1300 ms land shortly after a poll and are caught up. The read at 600 ms gets A from the poll sent at 0 ms although the gateway moved to B at 350 ms, and the reads at 1750 ms and 1950 ms both get C from the poll sent at 1500 ms although the gateway has been on D since 1550 ms. Whether a read is caught up is a matter of timing. The gateway's pre-confirmed state changes one to two times a second on mainnet (a new block about every two seconds, plus the transactions added in between), so about one read in four is one state behind.

- **What you get:** every read answers in about a millisecond (p50 0.7 ms; p99 about 1 ms, 3.5 ms in bursts); no read ever waits.
- **What it costs:** a flat 2.5 gateway requests per second whether or not anybody reads, and about one read in four one state behind at every load level: about 76% of sparse reads caught up, 78% at 5 reads per second, 79% at 20, 75% in bursts, with a staleness p90 of about 130 to 200 ms.

### Polling on demand without waiting: `1s / 250ms / 0s`

![Timeline of polling on demand without a wait: a read that finds data older than 250 ms triggers a poll but answers with the old data, and a burst of four reads shares one poll](/img/preconfirmed/timeline-on-demand-no-wait.svg)

The tick is 1 s, so a node nobody reads polls once a second. The fresh window is 250 ms: a read that finds the stored block older than that triggers a poll, but with no wait it does not wait for it. It answers at once with the old data, and the poll proceeds for whoever reads next. A successful read-triggered poll restarts the tick, so while reads keep arriving the tick is pushed back again and again and never fires: polling is driven by the reads alone.

In the timeline, the read at 600 ms finds data 480 ms old, triggers a poll and answers with A while the gateway is already on B. The read at 800 ms arrives 80 ms after that poll completed, inside the fresh window, and answers B at once, caught up. The burst at 1300 ms shares one poll (the first read triggers it, the other three join it) and all four answer B while the gateway is on C. The read at 1750 ms triggers a poll and answers C; the read at 1950 ms is the one that benefits and answers D at once. The hollow triangles are the ticks the read-triggered polls cancelled.

A poll helps the reads that follow it within 250 ms, never the read that triggered it. That suits a node that is read continuously and does nothing for a sparse reader, whose every read is the one that triggers the poll. Under load the tick stops mattering: at 20 reads per second polls run back to back, one every round trip plus 250 ms plus the gap to the next read, and the node is exactly as fresh as with the timer alone.

- **What you get:** as few gateway requests as a 1 s tick allows when nobody reads (1.6 per second, about 40% fewer than the 500 ms tick) and reads that still never wait (p99 about 1 ms, 4 ms in bursts).
- **What it costs:** for anyone who reads rarely or in bursts, a block up to a second old: 40% of sparse reads caught up with a staleness p90 of 700 ms, and 53% of burst reads, every read in a burst getting the old block. Under load, nothing gained: 78% caught up at 20 reads per second for 2.8 requests per second.

### Polling on demand and waiting for the result: `1s / 250ms / 300ms` (the default)

![Timeline of polling on demand with a 300 ms wait: the reads that trigger or join a poll wait one round trip and answer with the gateway's current state; reads inside the fresh window answer at once](/img/preconfirmed/timeline-on-demand-wait.svg)

Same tick and fresh window; the read that triggers or joins a poll waits for it, at most 300 ms. A poll is one round trip, so such a read answers about 120 ms later with the state the gateway had at that moment. Reads inside the fresh window still answer at once. A waiting read is released the instant its poll completes, or at once if the poll fails (a poll the gateway refused, a transport error), so the cap only bites when a poll is slow.

The polls in the timeline are the same as without the wait; what changes is who waits for them. The read at 600 ms waits 120 ms and gets B. The read at 800 ms answers B at once, inside the fresh window. The four reads of the burst wait together for one poll and all get C. The read at 1750 ms waits and gets D; the read at 1950 ms answers D at once. Every read is caught up, and the gateway was asked exactly as often as in the timeline without the wait.

- **What you get:** the fewest gateway requests when nobody reads (1.4 per second) and 85% to 100% of reads caught up at every load level: every sparse read, 87% at 5 reads per second, 95% at 20, 85% in bursts, with a staleness p90 of 0 ms for sparse reads and at 20 reads per second.
- **What it costs:** the reads that trigger or join a poll wait for it. A sparse read always does (p50 121 ms), and so does every read in a burst (p50 119 ms); at 5 reads per second the p50 is 68 ms. At 20 reads per second most reads land inside a fresh window and the p50 stays at 0.8 ms, but 44% of reads take more than 10 ms and 24% more than 100 ms. The p99 is up to about 300 ms, the cap, at every load level. At 20 reads per second the node also makes about 10% more requests than the timer alone (2.8 against 2.5 per second), because polls then run every round trip plus 250 ms.

### A faster timer with the wait: `500ms / 250ms / 300ms`

![Timeline of the 500 ms tick with a 300 ms wait: more reads land inside a fresh window and answer at once, and one of them is still one state behind because the gateway changed right after the poll](/img/preconfirmed/timeline-500-tick-wait.svg)

The fresh window and the wait are the default's; the tick is 500 ms. With a poll every 500 ms and a fresh window of 250 ms, a sparse read has about an even chance of landing inside a window and answering at once; the other half trigger or join a poll and wait. On a node nobody reads, subscriptions advance every 500 ms instead of every second.

In the timeline, the read at 600 ms arrives while the tick's poll is in flight, joins it and waits only 20 ms for B. The read at 800 ms and the burst at 1300 ms land 180 ms after a tick's poll and answer at once, caught up. The read at 1750 ms also answers at once, 130 ms after the poll sent at 1500 ms, but the gateway moved to D at 1550 ms, just after that poll was sent: the read is one state behind. The fresh window bounds the age of the data, not whether the gateway has changed since. The read at 1950 ms falls outside the window, polls, waits 120 ms and gets D.

- **What you get:** the default's freshness under load (at 20 reads per second the two measured the same) with fewer reads waiting: sparse reads 92% caught up at a p50 of 0.7 ms instead of 121 ms, bursts 97% at 66 ms instead of 119 ms, 5 reads per second 96% at 22 ms instead of 68 ms, and subscriptions that advance twice as often on a quiet node. In bursts and at 5 reads per second that is about ten caught-up points more than the default's 85% and 87%; run-to-run noise is about five points.
- **What it costs:** 2.4 gateway requests per second when nobody reads, one more per second than the default, and sparse reads caught up 92% of the time rather than every time.

### What each option changes

![Scatter plot of gateway requests per second against the share of caught-up reads for every configuration measured, for a sparse reader and for a node serving 20 reads per second](/img/preconfirmed/cost-vs-freshness.svg)

**The poll interval sets the idle cost and how soon a quiet node moves.** With no reads at all, the tick alone sets the gateway cost: about 2.5 requests per second at 500 ms, 1.4 to 1.6 at 1 s and 1.1 at 2 s (one poll per tick, plus one extra request whenever a poll finds a new block and first fetches the block it had, about once every two seconds whatever the tick). It also sets how soon subscriptions advance on a node nobody reads and, with no wait, how old the data a sparse read gets can be: up to a tick plus a round trip, which is why in the upper panel the configurations without a wait fall from 76% caught up at 500 ms to 40% at 1 s and 4% at 2 s. Under load the interval stops mattering: once reads arrive faster than the poll cycle, every poll is read-triggered and the tick never fires. In the lower panel `1s / 250ms / 0s` and `2s / 250ms / 0s` sit on the same spot (2.8 and 2.7 requests per second, 78% and 81% caught up), and so do `1s / 250ms / 300ms` and `500ms / 250ms / 300ms`.

**Stale-after sets the poll rate under load, and with it whether the gateway throttles your node.** While reads keep coming, the node polls once every round trip plus stale-after, plus the gap to the next read, so the fresh window sets cost and freshness at the same time. At 20 reads per second: `500ms` gave 2.0 requests per second with 40% of reads one state behind; `250ms` about 2.8 per second with 95% caught up (with the wait; 78% without it); `100ms` 3.8 per second, of which the gateway refused 12% (HTTP 429); `0s` 5.3 per second with 28% refused. A refused poll fails at once and does not advance the fresh window's clock, so the next read polls again: throttling makes a node poll more, not less, while its reads fall back to the stored block. Neither `100ms` nor `0s` was fresher than `250ms` with the wait (87% and 91% caught up against 95%). With no reads, stale-after changes nothing.

![Bar charts of caught-up share and read latency p99 for waits of 0, 150, 300 and 1000 ms at four load levels on the 1 s tick with stale-after 250 ms](/img/preconfirmed/wait-sweep.svg)

**The wait sets what the triggering read gets, not the gateway cost.** On the 1 s tick with a 250 ms fresh window, waits of 0, 150, 300 and 1000 ms all cost 1.5 to 1.6 requests per second with sparse reads and 2.7 to 2.8 at 20 reads per second: the reads trigger the same polls, the wait only decides whether they wait for them. What moves is the share of reads caught up and the latency tail, which the cap sets directly: a p99 of about 1 ms without a wait, 152 ms with 150 ms, up to 302 ms with 300 ms, and with 1000 ms a p99 of about 300 ms but a slowest read of 502 ms. Which reads wait is set by the load, not by the cap: every sparse read, every read in a burst, and at 20 reads per second the 44% that trigger or join a poll.

A poll normally takes one round trip, about 120 ms, but a poll that finds the gateway on a new block first fetches the block it had, a second round trip, and takes about 240 ms. At the 1 s tick a 150 ms cap timed out on about one poll in five, and the read then answers with the old data after waiting for nothing: 81% of sparse reads caught up against 100% with 300 ms, 87% against 95% at 20 reads per second. A 300 ms cap covers nearly every poll. A 1 s cap never gives up on a slow poll: it changes nothing for sparse reads or at 20 reads per second (96% and 97% caught up), lifts bursts and 5 reads per second to 98% and 97% against 85% and 87% (run-to-run noise is about five points), and stretches the slowest reads to half a second instead of 300 ms.

### Putting it together

The configurations of the three presets side by side, rounded; "no reads" is a node nobody reads, "sparse" one read every five seconds, "bursts" ten reads at once every two seconds. `500ms / 250ms / 0s`, the RPC Providers preset, behaves exactly like `500ms / 500ms / 0s` under load, where the tick never fires and stale-after sets the poll rate, and should for sparse reads too: its 50% comes from 22 reads taken while the gateway's state was changing faster than during any other sparse-read measurement.

| | `500ms / 500ms / 0s` | `1s / 250ms / 300ms` (default) | `500ms / 250ms / 300ms` | `500ms / 250ms / 0s` |
| --- | --- | --- | --- | --- |
| gateway requests/s: no reads / 20 reads/s | 2.5 / 2.5 | 1.4 / 2.8 | 2.4 / 2.8 | 2.5 / 2.5 |
| caught up: sparse / 20 reads/s / bursts | about 76% / 79% / 75% | 100% / 95% / 85% | 92% / 95% / 97% | 50% / 78% / 69% |
| staleness p90: sparse / 20 reads/s / bursts | about 200 / 140 / 130 ms | 0 / 0 / 99 ms | 0 / 0 / 0 ms | about 320 / 150 / 250 ms |
| read latency p50: sparse / 20 reads/s / bursts | 0.7 / 0.7 / 1.2 ms | 121 / 0.8 / 119 ms | 0.7 / 0.8 / 66 ms | 0.7 / 0.7 / 1.2 ms |
| read latency p99, worst load level | 3.5 ms | 302 ms | 301 ms | 3.4 ms |

**The default preset.** The measurements show a node that costs the gateway the least when nobody reads (1.4 requests per second), answers every sparse read and 95% of reads at 20 per second with the gateway's current state, and stays at about 2.8 requests per second at 20 reads per second, below the rate at which the gateway starts refusing requests. What it gives up is latency on the reads that trigger a poll, about 120 ms for a sparse read or a burst with a p99 of about 300 ms, and bursts are caught up 85% of the time rather than always. On a node nobody reads, subscriptions advance once a second.

**The RPC Providers preset.** Measured directly: every read answers in about a millisecond (p99 1.3 ms at 20 reads per second, 3.4 ms in bursts), the gateway cost is a flat 2.5 requests per second at every load level with no refused polls, and subscriptions advance at least every 500 ms. What it gives up is the triggering read's freshness: under load about one read in five is one state behind (78% caught up at 20 reads per second, staleness p90 about 150 ms), in bursts about one in three (69%), since a burst shares one poll and no read in it waits. The 1 s tick would save requests only on a node nobody reads, which a provider's node rarely is.

**The Bots and MEV preset.** `500ms / 250ms / 1s` itself was not measured. What follows is inferred from `1s / 250ms / 1s`, which shows what the 1 s wait gives, and from `500ms / 250ms / 300ms` against the default, which shows what the 500 ms tick changes. The 1 s wait gave 96% to 98% of reads caught up at every load level with a staleness p90 of 0 ms, not every read; its price is a latency tail to half a second (the slowest read at 20 reads per second took 502 ms) at the same gateway cost as the 300 ms wait, 2.8 requests per second at 20 reads per second. The 500 ms tick changes nothing under load, where the tick never fires; for sparse reads it lets about half answer at once instead of waiting a round trip, and it costs 2.4 requests per second when nobody reads. The combination should therefore give the freshness of the 1 s wait with the latency profile of the 500 ms tick. Stale-after stays at `250ms` because `100ms` and `0s` bought no freshness, only refused polls.

## Check that the gateway is not throttling your node

The gateway limits requests per IP address. In the measurements, refusals (HTTP 429) began at about six pre-confirmed requests per second from one address, counting the node's polls and the measurement's own reads of the gateway together, and became frequent from seven. The synchronizer's other gateway calls, about 1.5 per second, count against the same budget, and so does every other node or client behind the same address. Every preset above keeps a single node under that limit: at most about 2.8 pre-confirmed requests per second at 20 reads per second and, since polls cannot run faster than one per round trip plus stale-after, at most about 3 at any higher rate. Two nodes behind one address do not.

Verify on your node with the [metrics endpoint](monitoring). The refused polls are counted by

```
feeder_client_request_latency_count{method="/feeder_gateway/get_preconfirmed_block",status="429"}
```

which should stay at zero at your production poll rate; the same counter with `status="200"` is the poll rate itself. As a Prometheus query:

```
sum by (status) (rate(feeder_client_request_latency_count{method="/feeder_gateway/get_preconfirmed_block"}[5m]))
```

If the 429 count grows, lengthen `--preconfirmed-stale-after`, which sets the poll rate under load, or give the node its own egress IP before lowering any of the three values: a refused poll makes the next read poll again, so a throttled node polls more and answers staler, although its reads never fail. An API key passed with `--gw-api-key` is not evidence of headroom on its own: the key used in the measurements did not lift the per-IP limit.

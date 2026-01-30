---
title: Performance Tuning
---

It is important for full nodes to scale accordingly to the hardware where they are being executed. To unlock this, the following are a list of configurations users can update based on their hardware specs to maximize the performance of their Juno node.

The default values of each of these options are set to maximize performance with a machine that matches Juno's minimum requirements – described in the **Hardware Requirement** section.

## Database Compression

Set by the `--db-compression` flag, it applies a compression algorithm over the database **every time** Juno writes to it.

Available options:
- `snappy`: Fast compression with a low compression ratio
- `zstd`: Slower but reduces storage quite a lot
- `minlz`: Alternative compression option

Depending on the compression algorithm used it becomes a trade-off between **disk space** and **CPU** usage every time there is a disk operation.

We recommend `zstd` because it is fast enough that it doesn't delays any process significantly while providing huge database size reduction.

:::info
Note that once the compression is changed the new database is not compressed immediately, but gradually through the node usage by writing new information.
:::

:::info
There is a secret `zstd1` option that provides far greater performance than `zstd` but we are still testing it out and it might become the default later.
:::

## Database Memory Table Size

Set by the `--db-memtable-size` flag (default: 256 MB), this controls the amount of memory allocated for the database memtable. The memtable is an in-memory buffer where writes are stored before being flushed to disk.

A sensible default is **256 MB** for nodes that satisfy the minimum requirements. Increasing this value reduces the frequency of disk flushes, which can improve write throughput during sync.

:::warning
Setting this value too high can cause **uneven write performance**. Larger memtables mean flushes happen less frequently but involve more data at once, leading to bursty I/O patterns. If writes accumulate faster than the database can flush, writes will stall entirely until flushing catches up. A moderate value like 256 MB balances flush frequency with I/O smoothness.
:::

## Database Compaction Concurrency

Set by the `--db-compaction-concurrency` flag, this controls how many concurrent compaction workers the database uses. Compaction is the background process that merges and optimises data on disk.

Format options:
- `N`: Sets the range from 1 to N workers (e.g., `--db-compaction-concurrency=4`)
- `M,N`: Sets the range from M to N workers (e.g., `--db-compaction-concurrency=2,8`)

The default is `1,GOMAXPROCS/2` (half of available CPU cores). Increasing the upper bound on systems with many cores can speed up compaction as well as increase CPU resources usage while syncing parts of the network where there was a lot of usage.

:::info
Note that this effectively improve syncing speed while behind the tip of the chain but after reaching the latest block resource usage will gradually reduce back to minimums.
:::

## Database Cache Size

Set by the `--db-cache-size` flag (default: 1024 MB), this determines the amount of memory allocated for caching frequently accessed data from the database.

A larger cache reduces disk reads and improves query performance. On systems with ample memory, increasing this value (e.g., 2048 or 4096 MB) can significantly improve RPC response times and overall node performance.

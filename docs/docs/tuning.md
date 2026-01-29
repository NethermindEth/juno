---
title: Performance Tuning
---

It is important for full nodes to scale accordingly to the hardware where they are being executed. To unlock this, the following are a list of configurations users can update based on their hardware specs to maximize the performance of their Juno node.

This specs are set targetted at the minimum requirements of Juno, set in the **Hardware Requirement** section.


## Database compression

Set by the `--db-compression` flag it applies a compression algorithm over the database **every time** Juno writes to it. For example `snappy` is quick with a low compression ratio while `zstd` is slower but reduces storage quite a lot. 

Depending on the compression algorithm used it becomes a trade-off between **disk space** and **CPU** usage everytime there is a disk operation.

:::info
Note that once the compression is changed the new database is not compressed immedietaly, but gradually through the node usage by writing new information. 
:::

## Database Memory Table Size


## Datbase Compaction Workers


## Database Memory Buffer

---
title: Contract Compilation
description: "Cap the memory and CPU time of each Sierra class compilation, and how many Juno runs at once."
---

Juno compiles Sierra classes to CASM in isolated child processes. The flags below cap the resources each compilation uses and how many run at once.

## Per-compilation limits

:::warning
These limits are enforced by the kernel and only on **Linux**. On other operating systems they have no effect.
:::

Two flags cap the resources each compilation process may use, protecting the node from malicious classes that would otherwise consume CPU and memory without bound:

- `--max-compilation-memory` (default: 4096 MB): Maximum virtual memory (address space) a single compilation process may use. A compilation exceeding it is aborted.
- `--max-compilation-cpu-time` (default: 10 seconds): Maximum CPU time a single compilation process may consume. A compilation exceeding it is killed.

Setting either flag to `0` disables that limit. The defaults are generous for legitimate classes and only need raising if genuine compilations are being aborted.

:::info
The CPU time limit counts seconds of CPU actually consumed by the compilation process, not elapsed wall-clock time, so it is unaffected by how busy the rest of the machine is.
:::

## Compilation concurrency

- `--max-concurrent-compilations`(default: unset): controls how many compilations run at once. Any non-negative integer is used directly (`0` disables compilations). Left unset (the default), Juno derives a safe value so concurrent compilations cannot exhaust RAM:

```
limit = min((available_memory - node_memory_reserve) / max_compilation_memory, cpu_cores), at least 1.
```

- `--max-compilation-queue` (default: unset): How many requests wait once the concurrency limit is reached before new ones are rejected. Unset uses twice the concurrency limit (`0` disables the queue).

- `--node-memory-reserve` (default: 4096 MB): Memory kept for the rest of the node, excluded from the compilation budget.

The available memory respects container limits (cgroups), so inside a memory-capped container the value reflects the container, not the host. On non-Linux, where the per-compilation memory limit does not apply, the limit is simply the CPU core count.

The default limit is always at least 1: on a memory-tight node compilations, it runs one at a time rather than the node refusing to start.

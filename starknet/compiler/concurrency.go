package compiler

// ConcurrencyLimit returns how many compilations fit in memory, capped by maxConcurrency.
// Memory is in MB. A 0 maxMemoryPerCompilation means no memory limit.
// Returns 1 when no compilation fits memory.
func ConcurrencyLimit(
	maxConcurrency uint64,
	availableMemory uint64,
	nodeMemoryReserve uint64,
	maxMemoryPerCompilation uint64,
) uint64 {
	if maxMemoryPerCompilation == 0 {
		return max(1, maxConcurrency)
	}

	if availableMemory <= nodeMemoryReserve {
		return 1
	}

	fitInMemory := (availableMemory - nodeMemoryReserve) / maxMemoryPerCompilation
	if fitInMemory == 0 {
		return 1
	}

	return min(fitInMemory, maxConcurrency)
}

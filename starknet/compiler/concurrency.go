package compiler

import (
	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/pbnjay/memory"
)

const megabyte = 1 << 20

// AvailableMemoryMB returns the RAM this process can use, in MB.
// It checks if the memory is limited by the cgroup limit, otherwise it uses the host RAM.
func AvailableMemoryMB() uint64 {
	hostMemory := memory.TotalMemory()
	cgroupLimit, err := memlimit.FromCgroup()
	if err == nil && cgroupLimit > 0 && cgroupLimit < hostMemory {
		return cgroupLimit / megabyte
	}
	return hostMemory / megabyte
}

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

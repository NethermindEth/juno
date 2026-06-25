package compiler

import (
	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/pbnjay/memory"
)

const megabyte = 1 << 20

// AvailableMemoryMB returns the RAM this process can use, in MB.
// Inside a container it uses the cgroup limit, otherwise the host RAM.
func AvailableMemoryMB() uint64 {
	hostMemory := memory.TotalMemory()
	cgroupLimit, err := memlimit.FromCgroup()
	if err == nil && cgroupLimit > 0 && cgroupLimit < hostMemory {
		return cgroupLimit / megabyte
	}
	return hostMemory / megabyte
}

// ConcurrencyLimit returns how many compilations fit in memory, capped by maxParallel.
// Memory is in MB. A zero maxMemoryPerCompilation means no memory limit.
// Returns 0 when memory fits no compilation.
func ConcurrencyLimit(
	maxParallel uint,
	availableMemory uint64,
	nodeMemoryReserve uint64,
	maxMemoryPerCompilation uint64,
) uint {
	if maxMemoryPerCompilation == 0 {
		return maxParallel
	}
	if availableMemory <= nodeMemoryReserve {
		return 0
	}
	fitInMemory := uint((availableMemory - nodeMemoryReserve) / maxMemoryPerCompilation)
	return min(fitInMemory, maxParallel)
}

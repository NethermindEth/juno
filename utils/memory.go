package utils

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

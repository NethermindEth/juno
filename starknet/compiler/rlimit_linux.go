package compiler

import "golang.org/x/sys/unix"

const rlimitsSupported = true

// applyRLimits caps the CPU time (RLIMIT_CPU, seconds) and address
// space (RLIMIT_AS, bytes) of the process identified by pid. A zero
// value leaves the corresponding limit unchanged.
func applyRLimits(pid int, cpuSeconds, memoryBytes uint64) error {
	if cpuSeconds > 0 {
		limit := unix.Rlimit{Cur: cpuSeconds, Max: cpuSeconds}
		if err := unix.Prlimit(pid, unix.RLIMIT_CPU, &limit, nil); err != nil {
			return err
		}
	}
	if memoryBytes > 0 {
		limit := unix.Rlimit{Cur: memoryBytes, Max: memoryBytes}
		if err := unix.Prlimit(pid, unix.RLIMIT_AS, &limit, nil); err != nil {
			return err
		}
	}
	return nil
}

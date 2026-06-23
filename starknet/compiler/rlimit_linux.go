package compiler

import (
	"errors"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

const rlimitsSupported = true

// ApplySelfRLimits caps the CPU time (RLIMIT_CPU, seconds) and address
// space (RLIMIT_AS, bytes) of the calling process. A zero value leaves
// the corresponding limit unchanged.
//
// It is called by the compile-sierra child at startup, before any
// compilation work, so the limits are in force for the whole compile
// with no race window. unix.Setrlimit applies process-wide on Linux.
func ApplySelfRLimits(cpuSeconds, memoryBytes uint64) error {
	if cpuSeconds > 0 {
		limit := unix.Rlimit{Cur: cpuSeconds, Max: cpuSeconds}
		if err := unix.Setrlimit(unix.RLIMIT_CPU, &limit); err != nil {
			return err
		}
	}
	if memoryBytes > 0 {
		limit := unix.Rlimit{Cur: memoryBytes, Max: memoryBytes}
		if err := unix.Setrlimit(unix.RLIMIT_AS, &limit); err != nil {
			return err
		}
	}
	return nil
}

// CPURlimitExceeded reports whether err indicates the compile-sierra child was
// killed by the kernel for exceeding an applied rlimit: RLIMIT_CPU raises
// SIGXCPU and then SIGKILL. It turns the otherwise opaque "signal: killed"
// exec error into a diagnosable resource-limit message.
func CPURlimitExceeded(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	ws, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}
	return ws.Signaled() && (ws.Signal() == syscall.SIGKILL || ws.Signal() == syscall.SIGXCPU)
}

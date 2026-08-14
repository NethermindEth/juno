package utils

import "syscall"

// MaxFDLimit returns the current soft limit on open file descriptors for this
// process. The Go runtime raises it close to the hard limit at startup.
func MaxFDLimit() (uint64, error) {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return 0, err
	}
	return lim.Cur, nil
}

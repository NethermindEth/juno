//go:build !linux

package compiler

const rlimitsSupported = false

// applyRLimits is a no-op: per-process resource limits are only
// enforced on Linux.
func applyRLimits(int, uint64, uint64) error {
	return nil
}

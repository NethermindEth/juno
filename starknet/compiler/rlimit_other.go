//go:build !linux

package compiler

const rlimitsSupported = false

// ApplySelfRLimits is a no-op: per-process resource limits are only
// enforced on Linux.
func ApplySelfRLimits(uint64, uint64) error {
	return nil
}

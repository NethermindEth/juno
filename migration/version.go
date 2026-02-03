package migration

import (
	"fmt"
	"iter"
	"math/bits"
)

// SchemaVersion represents a schema version as a bitset.
// It uses a uint64 where each bit at index i represents whether migration i is included.
// Bits can only be set (never cleared), ensuring migrations are never "unapplied".
// Supports up to 64 migrations (bits 0-63).
type SchemaVersion uint64

// Union combines two SchemaVersions using bitwise OR.
func (sv SchemaVersion) Union(other SchemaVersion) SchemaVersion {
	return sv | other
}

// Set sets the bit at the given index.
func (sv *SchemaVersion) Set(index uint8) {
	*sv |= 1 << index
}

// Has returns true if the bit at the given index is set.
func (sv SchemaVersion) Has(index uint8) bool {
	return (sv & (1 << index)) != 0
}

// Contains returns true if sv contains all bits that are set in other.
func (sv SchemaVersion) Contains(other SchemaVersion) bool {
	return other.Difference(sv) == 0
}

// Difference returns a new SchemaVersion with bits that are set in sv but not in other.
func (sv SchemaVersion) Difference(other SchemaVersion) SchemaVersion {
	return sv &^ other
}

// HighestBit returns the highest set bit index (0-63).
// Returns -1 if no bits are set.
func (sv SchemaVersion) HighestBit() int {
	return bits.Len64(uint64(sv)) - 1
}

// Len returns the number of set bits in the SchemaVersion.
// This is the cardinality (size) of the set.
func (sv SchemaVersion) Len() int {
	return bits.OnesCount64(uint64(sv))
}

// Iter returns an iterator over all set bit indices in ascending order.
func (sv SchemaVersion) Iter() iter.Seq[uint8] {
	return func(yield func(uint8) bool) {
		n := uint64(sv)
		if n == 0 {
			return
		}

		// Find first set bit
		offset := bits.TrailingZeros64(n)
		idx := offset

		for idx < 64 {
			if !yield(uint8(idx)) {
				return
			}

			// Clear the bit we just processed by shifting past it
			n >>= offset + 1
			if n == 0 {
				return
			}

			// Find next set bit in the shifted value
			offset = bits.TrailingZeros64(n)
			idx += offset + 1
		}
	}
}

// String returns a string representation of the SchemaVersion.
func (sv SchemaVersion) String() string {
	return fmt.Sprintf("SchemaVersion(0b%064b)", sv)
}

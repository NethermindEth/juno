// Package maps contains proof witnesses for the concrete map shapes in the
// decoder entry manifest. These functions are not production decoder entries.
package maps

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

// FeltEntries exercises allocation, arbitrary key collisions and a possibly
// absent lookup with the exact key and value types used by StateDiff.Nonces.
func FeltEntries(k1, k2, query felt.Felt, v1, v2 *felt.Felt) (map[felt.Felt]*felt.Felt, *felt.Felt, bool) {
	m := make(map[felt.Felt]*felt.Felt)
	m[k1] = v1
	m[k2] = v2
	v, ok := m[query]
	return m, v, ok
}

// MigratedEntries checks both distinct named felt instantiations by value.
func MigratedEntries(k1, k2, query felt.SierraClassHash, v1, v2 felt.CasmClassHash) (map[felt.SierraClassHash]felt.CasmClassHash, felt.CasmClassHash, bool) {
	m := make(map[felt.SierraClassHash]felt.CasmClassHash)
	m[k1] = v1
	m[k2] = v2
	v, ok := m[query]
	return m, v, ok
}

// ResourceEntries checks named integer keys and struct values containing a
// pointer. Owning the map does not itself grant ownership of that pointee.
func ResourceEntries(k1, k2, query core.Resource, v1, v2 core.ResourceBounds) (map[core.Resource]core.ResourceBounds, core.ResourceBounds, bool) {
	m := make(map[core.Resource]core.ResourceBounds)
	m[k1] = v1
	m[k2] = v2
	v, ok := m[query]
	return m, v, ok
}

// StorageEntries checks that an inner map remains the same owned map after
// storing it in an outer map and updating it through an outer lookup.
func StorageEntries(address, k1, k2, query felt.Felt, v1, v2 *felt.Felt) (map[felt.Felt]map[felt.Felt]*felt.Felt, *felt.Felt, bool) {
	inner := make(map[felt.Felt]*felt.Felt)
	inner[k1] = v1
	outer := make(map[felt.Felt]map[felt.Felt]*felt.Felt)
	outer[address] = inner
	outer[address][k2] = v2
	v, ok := outer[address][query]
	return outer, v, ok
}

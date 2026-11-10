package maps_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	witness "github.com/NethermindEth/juno/verification/fastcbor-proof/feasibility/maps"
)

// Each limb must participate in key equality. Exercise collisions, misses and
// present nil values in actual Go while the array-equality model gap is open.
func TestMapWitnesses(t *testing.T) {
	keys := []felt.Felt{
		felt.Felt([4]uint64{}),
		felt.Felt([4]uint64{1, 0, 0, 0}),
		felt.Felt([4]uint64{0, 1, 0, 0}),
		felt.Felt([4]uint64{0, 0, 1, 0}),
		felt.Felt([4]uint64{0, 0, 0, 1}),
		felt.Felt([4]uint64{^uint64(0), ^uint64(0), ^uint64(0), ^uint64(0)}),
	}
	first := &keys[1]
	for i, k1 := range keys {
		for j, k2 := range keys {
			for q, query := range keys {
				wantLen := 2
				if i == j {
					wantLen = 1
				}
				present := q == i || q == j
				var want *felt.Felt
				if q == i && q != j {
					want = first
				}
				m, got, ok := witness.FeltEntries(k1, k2, query, first, nil)
				if len(m) != wantLen || got != want || ok != present {
					t.Fatalf("felt i=%d j=%d q=%d: len=%d value=%p present=%v", i, j, q, len(m), got, ok)
				}
				outer, got, ok := witness.StorageEntries(keys[5], k1, k2, query, first, nil)
				if len(outer) != 1 || len(outer[keys[5]]) != wantLen || got != want || ok != present {
					t.Fatalf("storage i=%d j=%d q=%d: value=%p present=%v", i, j, q, got, ok)
				}
				var wantClass felt.CasmClassHash
				if q == j {
					wantClass = felt.CasmClassHash(keys[2])
				} else if q == i {
					wantClass = felt.CasmClassHash(keys[1])
				}
				classes, class, ok := witness.MigratedEntries(
					felt.SierraClassHash(k1), felt.SierraClassHash(k2), felt.SierraClassHash(query),
					felt.CasmClassHash(keys[1]), felt.CasmClassHash(keys[2]),
				)
				if len(classes) != wantLen || class != wantClass || ok != present {
					t.Fatalf("migrated i=%d j=%d q=%d: len=%d present=%v", i, j, q, len(classes), ok)
				}
				v1 := core.ResourceBounds{MaxAmount: 17, MaxPricePerUnit: first}
				v2 := core.ResourceBounds{MaxAmount: ^uint64(0)}
				var wantBounds core.ResourceBounds
				if q == j {
					wantBounds = v2
				} else if q == i {
					wantBounds = v1
				}
				resources, bounds, ok := witness.ResourceEntries(core.Resource(i), core.Resource(j), core.Resource(q), v1, v2)
				if len(resources) != wantLen || bounds != wantBounds || ok != present {
					t.Fatalf("resources i=%d j=%d q=%d: len=%d bounds=%+v present=%v", i, j, q, len(resources), bounds, ok)
				}
			}
		}
	}
}

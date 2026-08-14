//go:build !race

package indexed_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/indexed"
	"github.com/stretchr/testify/require"
)

func TestLazySliceAllocations(t *testing.T) {
	lazySlice := newBenchLazySlice(t)

	t.Run("All allocates only the result slice", func(t *testing.T) {
		allocs := testing.AllocsPerRun(10, func() {
			if _, err := lazySlice.All(); err != nil {
				t.Fatal(err)
			}
		})
		require.Equal(t, 1.0, allocs)
	})

	t.Run("Iter allocates only the reused decode target", func(t *testing.T) {
		allocs := testing.AllocsPerRun(10, func() {
			for _, err := range lazySlice.Iter() {
				if err != nil {
					t.Fatal(err)
				}
			}
		})
		require.Equal(t, 1.0, allocs)
	})

	t.Run("AllMapped allocates the decode target and the result slice", func(t *testing.T) {
		allocs := testing.AllocsPerRun(10, func() {
			_, err := indexed.AllMapped(
				lazySlice,
				func(_ int, value benchItem) ([4]uint64, error) { return value.Hash, nil },
			)
			if err != nil {
				t.Fatal(err)
			}
		})
		require.Equal(t, 2.0, allocs)
	})

	t.Run("Pointer into extract's value costs one copy per element", func(t *testing.T) {
		allocs := testing.AllocsPerRun(10, func() {
			_, err := indexed.AllMapped(
				lazySlice,
				func(_ int, value benchItem) (*[4]uint64, error) { return &value.Hash, nil },
			)
			if err != nil {
				t.Fatal(err)
			}
		})
		require.Equal(t, float64(benchItemsCount)+2, allocs)
	})
}

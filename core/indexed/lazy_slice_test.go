package indexed_test

import (
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core/indexed"
	"github.com/NethermindEth/juno/encoder"
	"github.com/stretchr/testify/require"
)

type fullShape struct {
	A string
	B int64
}

type partialShape struct {
	A string
}

type nullableShape struct {
	A string
	B *int64
}

type pointerShape struct {
	V *int64
}

func ptrTo[T any](v T) *T { return &v }

func rawLazySlice[T any](t *testing.T, items ...any) indexed.LazySlice[T] {
	t.Helper()
	indexes := make([]int, len(items))
	var data []byte
	for i, item := range items {
		encoded, err := encoder.Marshal(item)
		require.NoError(t, err)
		indexes[i] = len(data)
		data = append(data, encoded...)
	}
	return indexed.NewLazySlice[T](indexes, data)
}

func TestAllMapped(t *testing.T) {
	t.Run("Success with index", func(t *testing.T) {
		lazySlice := rawLazySlice[fullShape](
			t,
			fullShape{A: "x", B: 5},
			fullShape{A: "y", B: 7},
			fullShape{A: "z", B: 9},
		)

		indexes := make([]int, 0, 3)
		results, err := indexed.AllMapped(
			lazySlice,
			func(index int, value fullShape) (int64, error) {
				indexes = append(indexes, index)
				return value.B, nil
			},
		)
		require.NoError(t, err)
		require.Equal(t, []int64{5, 7, 9}, results)
		require.Equal(t, []int{0, 1, 2}, indexes)
	})

	t.Run("Extract error propagates", func(t *testing.T) {
		lazySlice := rawLazySlice[fullShape](
			t,
			fullShape{A: "x", B: 5},
			fullShape{A: "y", B: 7},
		)

		expectedErr := errors.New("extract failed")
		_, err := indexed.AllMapped(
			lazySlice,
			func(index int, _ fullShape) (int64, error) {
				if index == 1 {
					return 0, expectedErr
				}
				return 0, nil
			},
		)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("Decode error propagates", func(t *testing.T) {
		lazySlice := indexed.NewLazySlice[fullShape]([]int{0}, []byte{0xff, 0xff})
		_, err := indexed.AllMapped(
			lazySlice,
			func(_ int, value fullShape) (int64, error) { return value.B, nil },
		)
		require.Error(t, err)
	})
}

func TestReusedDecodeTargetIsReset(t *testing.T) {
	t.Run("Absent key decodes as zero", func(t *testing.T) {
		lazySlice := rawLazySlice[fullShape](
			t,
			fullShape{A: "x", B: 5},
			partialShape{A: "y"},
		)

		results, err := indexed.AllMapped(
			lazySlice,
			func(_ int, value fullShape) (fullShape, error) { return value, nil },
		)
		require.NoError(t, err)
		require.Equal(t, []fullShape{{A: "x", B: 5}, {A: "y", B: 0}}, results)
	})

	t.Run("CBOR null decodes as zero", func(t *testing.T) {
		lazySlice := rawLazySlice[fullShape](
			t,
			fullShape{A: "x", B: 5},
			nullableShape{A: "y", B: nil},
		)

		results, err := indexed.AllMapped(
			lazySlice,
			func(_ int, value fullShape) (fullShape, error) { return value, nil },
		)
		require.NoError(t, err)
		require.Equal(t, []fullShape{{A: "x", B: 5}, {A: "y", B: 0}}, results)
	})

	t.Run("Iter resets too", func(t *testing.T) {
		lazySlice := rawLazySlice[fullShape](
			t,
			fullShape{A: "x", B: 5},
			partialShape{A: "y"},
		)

		items := make([]fullShape, 0, 2)
		for item, err := range lazySlice.Iter() {
			require.NoError(t, err)
			items = append(items, item)
		}
		require.Equal(t, []fullShape{{A: "x", B: 5}, {A: "y", B: 0}}, items)
	})
}

func TestAllMappedPointerFieldFreshPointees(t *testing.T) {
	lazySlice := rawLazySlice[pointerShape](
		t,
		pointerShape{V: ptrTo[int64](1)},
		pointerShape{V: ptrTo[int64](2)},
	)

	results, err := indexed.AllMapped(
		lazySlice,
		func(_ int, value pointerShape) (*int64, error) { return value.V, nil },
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), *results[0])
	require.Equal(t, int64(2), *results[1])
	require.NotSame(t, results[0], results[1])
}

func TestAllMappedPointerIntoValueStaysCorrect(t *testing.T) {
	lazySlice := rawLazySlice[fullShape](
		t,
		fullShape{A: "x", B: 5},
		fullShape{A: "y", B: 7},
	)

	results, err := indexed.AllMapped(
		lazySlice,
		func(_ int, value fullShape) (*int64, error) { return &value.B, nil },
	)
	require.NoError(t, err)
	require.Equal(t, int64(5), *results[0])
	require.Equal(t, int64(7), *results[1])
	require.NotSame(t, results[0], results[1])
}

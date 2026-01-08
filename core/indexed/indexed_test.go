package indexed_test

import (
	cryptorand "crypto/rand"
	"errors"
	"iter"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core/indexed"
	"github.com/NethermindEth/juno/db"
	"github.com/stretchr/testify/require"
)

const itemsCount = 100

func generate[T any](
	t *testing.T,
	random func() T,
	count int,
	err error,
) ([]T, iter.Seq2[T, error]) {
	t.Helper()
	items := make([]T, count)
	for i := range items {
		items[i] = random()
	}

	return items, func(yield func(T, error) bool) {
		for _, item := range items {
			if !yield(item, err) {
				return
			}
		}

		if err != nil {
			yield(*new(T), err)
		}
	}
}

func writeToBufferedEncoderSuccessfully[T any](
	t *testing.T,
	bufferedEncoder indexed.BufferedEncoder,
	items iter.Seq2[T, error],
	count int,
) []int {
	t.Helper()
	indexes, err := indexed.Write(bufferedEncoder, items)
	require.NoError(t, err)
	require.Len(t, indexes, count)

	for i := 1; i < len(indexes); i++ {
		require.Less(t, indexes[i-1], indexes[i])
	}
	require.Less(t, indexes[len(indexes)-1], bufferedEncoder.Len())

	return indexes
}

func writeUnsuccessfully[T any](
	t *testing.T,
	items iter.Seq2[T, error],
	expectedError error,
) {
	t.Helper()
	bufferedEncoder := indexed.NewBufferedEncoder()
	require.Equal(t, 0, bufferedEncoder.Len())
	require.Len(t, bufferedEncoder.Buffer.Bytes(), 0)

	_, err := indexed.Write(bufferedEncoder, items)
	require.Error(t, err)
	require.Equal(t, expectedError, err)
}

func assertLazySlice[T any](t *testing.T, expected []T, lazySlice indexed.LazySlice[T]) {
	t.Helper()
	t.Run("Get", func(t *testing.T) {
		for i := range expected {
			item, err := lazySlice.Get(i)
			require.NoError(t, err)
			require.Equal(t, expected[i], item)
		}
	})

	t.Run("Get out of range", func(t *testing.T) {
		_, err := lazySlice.Get(len(expected))
		require.Error(t, err)
		require.Equal(t, db.ErrKeyNotFound, err)

		_, err = lazySlice.Get(-1)
		require.Error(t, err)
		require.Equal(t, db.ErrKeyNotFound, err)
	})

	t.Run("All", func(t *testing.T) {
		items, err := lazySlice.All()
		require.NoError(t, err)
		require.Equal(t, expected, items)
	})
}

func TestIndexed(t *testing.T) {
	t.Run("Error", func(t *testing.T) {
		expectedError := errors.New("test error")
		_, seq := generate(t, rand.Int32, itemsCount, expectedError)
		writeUnsuccessfully(t, seq, expectedError)
	})

	t.Run("Empty", func(t *testing.T) {
		assertLazySlice(
			t,
			[]string{},
			indexed.NewLazySlice[string]([]int{}, []byte{}),
		)
	})

	t.Run("Success", func(t *testing.T) {
		stringItems, stringSeq := generate(t, cryptorand.Text, itemsCount, nil)
		int64Items, int64Seq := generate(t, rand.Int64, itemsCount, nil)

		bufferedEncoder := indexed.NewBufferedEncoder()
		require.Equal(t, 0, bufferedEncoder.Len())
		require.Len(t, bufferedEncoder.Buffer.Bytes(), 0)

		var (
			stringIndexes []int
			int64Indexes  []int
		)

		t.Run("Write first index", func(t *testing.T) {
			stringIndexes = writeToBufferedEncoderSuccessfully(t, bufferedEncoder, stringSeq, itemsCount)
		})

		t.Run("Write second index", func(t *testing.T) {
			int64Indexes = writeToBufferedEncoderSuccessfully(t, bufferedEncoder, int64Seq, itemsCount)
		})

		t.Run("Assert first index", func(t *testing.T) {
			bytes := bufferedEncoder.Bytes()
			assertLazySlice(
				t,
				stringItems,
				indexed.NewLazySlice[string](
					stringIndexes,
					bytes[:int64Indexes[0]],
				),
			)
		})

		t.Run("Assert second index", func(t *testing.T) {
			assertLazySlice(
				t,
				int64Items,
				indexed.NewLazySlice[int64](
					int64Indexes,
					bufferedEncoder.Bytes(),
				),
			)
		})
	})
}

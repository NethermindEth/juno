package partial_test

import (
	"math/rand/v2"
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/partial"
	"github.com/NethermindEth/juno/db/typed/value"
	"github.com/stretchr/testify/require"
)

const (
	keyCount       = 1000
	maxSubKeyCount = 100
)

var readCounter atomic.Int32

type itemSerializer struct{}

func (itemSerializer) Marshal(v *uint64) ([]byte, error) {
	return value.Cbor[uint64]().Marshal(v)
}

func (itemSerializer) Unmarshal(data []byte, v *uint64) error {
	readCounter.Add(1)
	if err := value.Cbor[uint64]().Unmarshal(data, v); err != nil {
		return err
	}
	return nil
}

type arraySerializer[I any, IS value.Serializer[I]] struct{}

func (arraySerializer[I, IS]) Marshal(v *[]I) ([]byte, error) {
	encoded := make([][]byte, len(*v))
	var err error
	for i := range *v {
		if encoded[i], err = (IS{}).Marshal(&(*v)[i]); err != nil {
			return nil, err
		}
	}

	return value.Cbor[[][]byte]().Marshal(&encoded)
}

func (arraySerializer[I, IS]) Unmarshal(data []byte, v *[]I) error {
	var encoded [][]byte
	if err := value.Cbor[[][]byte]().Unmarshal(data, &encoded); err != nil {
		return err
	}
	*v = make([]I, len(encoded))
	for i := range encoded {
		if err := (IS{}).Unmarshal(encoded[i], &(*v)[i]); err != nil {
			return err
		}
	}
	return nil
}

type arrayPartialSerializer[I any, IS value.Serializer[I]] struct{}

func (arrayPartialSerializer[I, IS]) UnmarshalPartial(subKey int, data []byte, v *I) error {
	var encoded [][]byte
	if err := value.Cbor[[][]byte]().Unmarshal(data, &encoded); err != nil {
		return err
	}
	return IS{}.Unmarshal(encoded[subKey], v)
}

var (
	bucket = typed.NewBucket(
		db.Bucket(0),
		key.Uint64,
		arraySerializer[uint64, itemSerializer]{},
	)
	partialBucket = partial.NewPartialBucket(
		bucket,
		arrayPartialSerializer[uint64, itemSerializer]{},
	)
)

func generateItems() [][]uint64 {
	items := make([][]uint64, keyCount)
	current := uint64(0)
	for key := range keyCount {
		subKeyCount := rand.IntN(maxSubKeyCount)
		items[key] = make([]uint64, subKeyCount)
		for subKey := range subKeyCount {
			items[key][subKey] = current
			current++
		}
	}
	return items
}

func runTest(
	t *testing.T,
	database db.KeyValueReader,
	items [][]uint64,
	needsToReadEntireValue bool,
	get func(db.KeyValueReader, uint64, int) (uint64, error),
) {
	for key, values := range items {
		key := uint64(key)
		expectedReads := 1
		if needsToReadEntireValue {
			expectedReads = len(values)
		}

		for subKey, expected := range values {
			readCounter.Store(0)
			actual, err := get(database, key, subKey)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
			require.Equal(t, expectedReads, int(readCounter.Load()))
		}
	}
}

func TestPartialBucket(t *testing.T) {
	database, err := pebblev2.New(t.TempDir())
	require.NoError(t, err)
	defer database.Close()

	items := generateItems()

	t.Run("Insert entries", func(t *testing.T) {
		for key, values := range items {
			key := uint64(key)
			require.NoError(t, bucket.Put(database, key, &values))
		}
	})

	t.Run("Get full needs to read the entire value", func(t *testing.T) {
		runTest(
			t,
			database,
			items,
			true,
			func(database db.KeyValueReader, key uint64, subKey int) (uint64, error) {
				v, err := bucket.Get(database, key)
				if err != nil {
					return 0, err
				}
				return v[subKey], nil
			},
		)
	})

	t.Run("Get partial does not need to read the entire value", func(t *testing.T) {
		runTest(t, database, items, false, partialBucket.Get)
	})
}

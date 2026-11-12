package value_test

import (
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/value"
	"github.com/stretchr/testify/require"
)

type record struct {
	A     uint64
	Names []string
	Inner *struct{ X uint64 }
}

// loose has a field of an interface type that nothing registered implements, which only fxamacker
// reads.
type loose struct {
	N uint64
	V any
}

func TestFastCbor(t *testing.T) {
	database := memory.New()

	t.Run("a value is read as it was written", func(t *testing.T) {
		bucket := typed.NewBucket(db.Bucket(0), key.Uint64, value.FastCbor[record]())
		want := record{A: 7, Names: []string{"a", "b"}, Inner: &struct{ X uint64 }{X: 9}}
		require.NoError(t, bucket.Put(database, 1, &want))
		got, err := bucket.Get(database, 1)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("a value of a shape it cannot read is still read", func(t *testing.T) {
		bucket := typed.NewBucket(db.Bucket(1), key.Uint64, value.FastCbor[loose]())
		want := loose{N: 3, V: []any{uint64(1), "a"}}
		require.NoError(t, bucket.Put(database, 1, &want))
		got, err := bucket.Get(database, 1)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}

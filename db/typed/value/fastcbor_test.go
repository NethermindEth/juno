package value_test

import (
	"sync/atomic"
	"testing"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/db/typed"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/NethermindEth/juno/db/typed/value"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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

// warnings counts the warnings it is given.
type warnings struct{ n atomic.Int64 }

func (w *warnings) Warn(string, ...zap.Field) { w.n.Add(1) }
func (*warnings) Debug(string, ...zap.Field)  {}
func (*warnings) Info(string, ...zap.Field)   {}
func (*warnings) Error(string, ...zap.Field)  {}
func (*warnings) Trace(string, ...zap.Field)  {}

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

	t.Run("a read it hands over is reported, and one it does not is not", func(t *testing.T) {
		reports := new(warnings)
		value.SetFastCborLogger(reports)
		t.Cleanup(func() { value.SetFastCborLogger(&warnings{}) })

		serializer := value.FastCbor[loose]()
		data, err := serializer.Marshal(&loose{N: 3, V: []any{uint64(1)}})
		require.NoError(t, err)
		var got loose
		require.NoError(t, serializer.Unmarshal(data, &got))
		require.Equal(t, int64(1), reports.n.Load())

		plain := value.FastCbor[record]()
		data, err = plain.Marshal(&record{A: 1})
		require.NoError(t, err)
		var read record
		require.NoError(t, plain.Unmarshal(data, &read))
		require.Equal(t, int64(1), reports.n.Load())
	})

	t.Run("the first item is read and the rest returned", func(t *testing.T) {
		first, err := cbor.Marshal(&record{A: 5})
		require.NoError(t, err)
		var got record
		rest, err := value.FastCbor[record]().UnmarshalFirst(append(first, 0x07), &got)
		require.NoError(t, err)
		require.Equal(t, uint64(5), got.A)
		require.Equal(t, []byte{0x07}, rest)
	})

	t.Run("bytes after the value are an error", func(t *testing.T) {
		bucket := typed.NewBucket(db.Bucket(2), key.Uint64, value.FastCbor[uint64]())
		require.NoError(t, database.Put(db.Bucket(2).Key(key.Uint64.Marshal(1)), []byte{0x05, 0x01}))
		_, err := bucket.Get(database, 1)
		require.ErrorContains(t, err, "extraneous data")
	})
}

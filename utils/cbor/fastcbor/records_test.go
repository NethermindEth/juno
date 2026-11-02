package fastcbor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// corpus holds mainnet records of every kind and several sizes, as the database stores them. The
// tests that use it skip where it is not.
const corpus = "/mnt/dev/juno-benchmark/cbor-fast-iter/corpus"

// byteString puts the head of a CBOR byte string before data, as a class is stored: the class is
// the contents of a byte string, because DeclaredClassDefinition writes its own binary encoding.
func byteString(data []byte) []byte {
	n := len(data)
	var head []byte
	switch {
	case n < 24:
		head = []byte{0x40 | byte(n)}
	case n < 1<<8:
		head = []byte{0x58, byte(n)}
	case n < 1<<16:
		head = []byte{0x59, byte(n >> 8), byte(n)}
	default:
		head = []byte{0x5a, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
	}
	return append(head, data...)
}

func TestMainnetRecords(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(corpus, "*.bin"))
	if len(files) == 0 {
		t.Skip("no corpus")
	}
	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".bin")
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		switch {
		case strings.HasPrefix(name, "receipt"):
			t.Run(name, func(t *testing.T) { readByFast[*core.TransactionReceipt](t, data) })
		case strings.HasPrefix(name, "header"):
			t.Run(name, func(t *testing.T) { readByFast[core.Header](t, data) })
		case strings.HasPrefix(name, "state"):
			t.Run(name, func(t *testing.T) { readByFast[core.StateUpdate](t, data) })
		case strings.HasPrefix(name, "transaction"):
			t.Run(name, func(t *testing.T) { readByFast[core.Transaction](t, data) })
		case strings.HasPrefix(name, "class"):
			t.Run(name, func(t *testing.T) { readByFast[core.DeclaredClassDefinition](t, byteString(data)) })
			// The class inside is read by the fastcbor package whichever way the record is, so it is
			// also compared on its own, after the 8 bytes of the block it was declared at.
			t.Run(name+"/class", func(t *testing.T) { readByFast[core.ClassDefinition](t, data[8:]) })
		}
	}
}

func TestWhatItCannotReadIsLeftToFxamacker(t *testing.T) {
	// An interface that no registered type implements has no reader.
	type record struct {
		X any
		N uint64
	}
	require.True(t, compare[record](t, marshal(t, record{X: map[string]any{"a": uint64(1)}, N: 3})))
}

func TestADestinationThatIsNotZeroIsLeftToFxamacker(t *testing.T) {
	type record struct {
		A uint64
		B string
		C []uint64
	}
	data := marshal(t, record{A: 1, B: "x", C: []uint64{1, 2}})
	prefilled := func() record { return record{A: 9, C: []uint64{7, 8, 9, 10}} }

	got, expected := prefilled(), prefilled()
	before := fell.n.Load()
	require.NoError(t, fastcbor.Unmarshal(data, &got))
	require.NoError(t, fxamacker.Unmarshal(data, &expected))
	require.Equal(t, expected, got)
	require.Equal(t, before, fell.n.Load(), "handing it over is not a failure")
}

func TestTrailingBytesAreAnError(t *testing.T) {
	data := append(marshal(t, uint64(5)), 0x01)
	var got uint64
	require.ErrorContains(t, fastcbor.Unmarshal(data, &got), "extraneous data")
	require.Zero(t, got)

	rest, err := fastcbor.UnmarshalFirst(data, &got)
	require.NoError(t, err)
	require.Equal(t, uint64(5), got)
	require.Equal(t, []byte{0x01}, rest)
}

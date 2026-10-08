package cbor_test

import (
	"testing"

	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// pair is a type only this test routes, stored as the array [X, Name].
type pair struct {
	_    struct{} `cbor:",toarray"`
	X    uint64
	Name string
	Tags []string
}

// unsupportedX is an X the reader leaves to fxamacker, after writing it.
const unsupportedX = 7

func readPair(r *fast.Reader, dst *pair) error {
	n, err := r.ArrayLen()
	if err != nil {
		return err
	}
	if n != 3 {
		return fast.ErrUnsupported
	}
	if dst.X, err = r.Uint64(); err != nil {
		return err
	}
	if dst.X == unsupportedX {
		return fast.ErrUnsupported
	}
	if dst.Name, err = r.String(); err != nil {
		return err
	}
	if !r.Null() {
		return fast.ErrUnsupported
	}
	r.Leave()
	return nil
}

//nolint:gochecknoinits
func init() {
	cbor.RegisterReader(readPair)
}

const pairName = "cbor_test.pair"

func pairStats() cbor.RouteStats { return cbor.Stats()[pairName] }

func encodePair(t *testing.T, p *pair) []byte {
	t.Helper()
	data, err := cbor.Marshal(p)
	require.NoError(t, err)
	return data
}

func TestRouteReadsAZeroDestination(t *testing.T) {
	data := encodePair(t, &pair{X: 3, Name: "three"})

	before := pairStats()
	var got pair
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.Equal(t, pair{X: 3, Name: "three"}, got)
	require.Equal(t, before.Routed+1, pairStats().Routed)
}

func TestRouteReadsThroughAPointerToPointer(t *testing.T) {
	data := encodePair(t, &pair{X: 3, Name: "three"})

	before := pairStats()
	var got *pair
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.Equal(t, &pair{X: 3, Name: "three"}, got)

	got = nil
	require.NoError(t, cbor.Unmarshal([]byte{0xf6}, &got))
	require.Nil(t, got)
	require.Equal(t, before.Routed+2, pairStats().Routed)
}

// fxamacker decodes into a destination that holds a value by updating it, which the
// readers do not copy, so such a destination goes to fxamacker.
func TestRouteLeavesANonZeroDestinationToFxamacker(t *testing.T) {
	data := encodePair(t, &pair{X: 3, Name: "three"})

	before := pairStats()
	got := pair{Tags: []string{"kept"}}
	want := pair{Tags: []string{"kept"}}
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.NoError(t, fxamacker.Unmarshal(data, &want))
	require.Equal(t, want, got)

	gotPtr := &pair{Name: "old"}
	wantPtr := &pair{Name: "old"}
	require.NoError(t, cbor.Unmarshal(data, &gotPtr))
	require.NoError(t, fxamacker.Unmarshal(data, &wantPtr))
	require.Equal(t, wantPtr, gotPtr)

	after := pairStats()
	require.Equal(t, before.NonZeroTarget+2, after.NonZeroTarget)
	require.Equal(t, before.Routed, after.Routed)
}

// A value is zero by Go's rules, as reflect.Value.IsZero has them, even where its memory is
// not all zero bytes, like an empty string that still points into another one.
func TestRouteTakesAnEmptyStringWithAPointerAsZero(t *testing.T) {
	data := encodePair(t, &pair{X: 3, Name: "three"})
	text := "text"

	before := pairStats()
	got := pair{Name: text[:0]}
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.Equal(t, pair{X: 3, Name: "three"}, got)
	require.Equal(t, before.Routed+1, pairStats().Routed)
}

func TestRouteRedoesAnUnsupportedFormInFxamacker(t *testing.T) {
	for name, data := range map[string][]byte{
		"a tag list":     encodePair(t, &pair{X: 3, Name: "three", Tags: []string{"tag"}}),
		"an X it leaves": encodePair(t, &pair{X: unsupportedX, Name: "seven"}),
	} {
		t.Run(name, func(t *testing.T) {
			before := pairStats()
			var got, want pair
			require.NoError(t, cbor.Unmarshal(data, &got))
			require.NoError(t, fxamacker.Unmarshal(data, &want))
			require.Equal(t, want, got)

			var gotPtr, wantPtr *pair
			require.NoError(t, cbor.Unmarshal(data, &gotPtr))
			require.NoError(t, fxamacker.Unmarshal(data, &wantPtr))
			require.Equal(t, wantPtr, gotPtr)

			after := pairStats()
			require.Equal(t, before.Unsupported+2, after.Unsupported)
			require.Equal(t, before.Routed, after.Routed)
		})
	}
}

func TestRouteReportsATypeMismatchAsUnmarshalTypeError(t *testing.T) {
	data := []byte{0x83, 0x61, 'x', 0x61, 'y', 0xf6} // ["x", "y", null]

	var got pair
	err := cbor.Unmarshal(data, &got)
	var typeErr *cbor.UnmarshalTypeError
	require.ErrorAs(t, err, &typeErr)
	require.Equal(t, pairName, typeErr.GoType)
	require.Equal(t, pair{}, got)
}

func TestRouteClearsTheDestinationOnAnError(t *testing.T) {
	data := encodePair(t, &pair{X: 3, Name: "three"})
	truncated := data[:len(data)-2]

	var got pair
	require.ErrorIs(t, cbor.Unmarshal(truncated, &got), fast.ErrTruncated)
	require.Equal(t, pair{}, got)

	var gotPtr *pair
	require.ErrorIs(t, cbor.Unmarshal(truncated, &gotPtr), fast.ErrTruncated)
	require.Nil(t, gotPtr)
}

func TestRouteChecksWhatFollowsTheItem(t *testing.T) {
	data := encodePair(t, &pair{X: 3, Name: "three"})
	withRest := append(append([]byte(nil), data...), 0x01, 0x02)

	var got pair
	require.EqualError(t, cbor.Unmarshal(withRest, &got), "cbor: 2 bytes of extraneous data")
	require.Error(t, fxamacker.Unmarshal(withRest, new(pair)))

	got = pair{}
	rest, err := cbor.UnmarshalFirst(withRest, &got)
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02}, rest)
	require.Equal(t, pair{X: 3, Name: "three"}, got)
}

func TestRouteCanBeTurnedOff(t *testing.T) {
	data := encodePair(t, &pair{X: 3, Name: "three"})

	cbor.SetFastEnabled(false)
	t.Cleanup(func() { cbor.SetFastEnabled(true) })
	before := pairStats()
	var got pair
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.Equal(t, pair{X: 3, Name: "three"}, got)
	require.Equal(t, before, pairStats())
}

func TestUnroutedTypesGoToFxamacker(t *testing.T) {
	type other struct{ A uint64 }
	data, err := cbor.Marshal(other{A: 5})
	require.NoError(t, err)

	var got other
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.Equal(t, other{A: 5}, got)

	// A pair by value is not a destination a route takes.
	require.Error(t, cbor.Unmarshal(data, pair{}))
}

func BenchmarkRoute(b *testing.B) {
	data, err := cbor.Marshal(&pair{X: 3, Name: "three"})
	require.NoError(b, err)
	b.Run("wrapper", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			var dst pair
			if err := cbor.Unmarshal(data, &dst); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("direct", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			var dst pair
			r := fast.NewReader(data)
			if err := readPair(&r, &dst); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("unrouted", func(b *testing.B) {
		type other struct{ X uint64 }
		small := []byte{0xa1, 0x61, 'X', 0x01}
		b.ReportAllocs()
		for range b.N {
			var dst other
			if err := cbor.Unmarshal(small, &dst); err != nil {
				b.Fatal(err)
			}
		}
	})
}

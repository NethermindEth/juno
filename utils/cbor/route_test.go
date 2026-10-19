package cbor_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/stretchr/testify/require"
)

// pair is a type only this test routes, stored as the array [X, Name].
type pair struct {
	_    struct{} `cbor:",toarray"`
	X    uint64
	Name string
	Tags []string
}

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

func encodePair(t *testing.T, p *pair) []byte {
	t.Helper()
	data, err := cbor.Marshal(p)
	require.NoError(t, err)
	return data
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

// Decodes running at once share the pool of Readers, so each must see only its own input.
func TestRouteDecodesConcurrently(t *testing.T) {
	const workers, rounds = 8, 200
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := range workers {
		wg.Go(func() {
			want := pair{X: uint64(w), Name: strings.Repeat("n", w)}
			data, err := cbor.Marshal(&want)
			if err != nil {
				errs <- err
				return
			}
			for range rounds {
				var got pair
				if err := cbor.Unmarshal(data, &got); err != nil {
					errs <- err
					return
				}
				if got.X != want.X || got.Name != want.Name {
					errs <- fmt.Errorf("worker %d decoded %+v", w, got)
					return
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
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

// The first read of a type to fall back is reported once for each reason, and nothing is reported
// before a logger is set.
func TestFirstFallbackOfATypeIsReportedOnce(t *testing.T) {
	type logged struct{ X uint64 }
	cbor.RegisterReader(func(*fast.Reader, *logged) error { return fast.ErrUnsupported })
	data, err := cbor.Marshal(&logged{X: 1})
	require.NoError(t, err)

	read := func() {
		var zero logged
		require.NoError(t, cbor.Unmarshal(data, &zero))
		nonZero := logged{X: 2}
		require.NoError(t, cbor.Unmarshal(data, &nonZero))
	}

	read() // no logger yet
	var mu sync.Mutex
	var reports []string
	cbor.SetFallbackLogger(func(typeName string, reason cbor.Reason) {
		mu.Lock()
		defer mu.Unlock()
		reports = append(reports, typeName+": "+reason.String())
	})
	t.Cleanup(func() { cbor.SetFallbackLogger(nil) })

	read()
	read()
	require.Equal(t, []string{
		"cbor_test.logged: unsupported form",
		"cbor_test.logged: non-zero destination",
	}, reports)
}

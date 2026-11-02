package fastcbor_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// The types of these tests are read for the first time here, and by many goroutines at once, so the
// first read of each plans it while others ask for it.
type (
	firstA struct {
		N     uint64
		Names []string
		Felts []*felt.Felt
		M     map[uint32]uint64
	}
	firstB struct {
		X *firstA
		Y *firstA
		Z map[felt.Felt]*felt.Felt
	}
	firstC struct {
		Items []firstB
		Tag   string
	}
)

func TestFirstReadsOfATypeByManyGoroutines(t *testing.T) {
	a := firstA{
		N: 1, Names: []string{"a", "b"},
		Felts: []*felt.Felt{felt.NewFromUint64[felt.Felt](3), felt.NewFromUint64[felt.Felt](4)},
		M:     map[uint32]uint64{1: 10, 2: 20},
	}
	b := firstB{X: &a, Y: &a, Z: map[felt.Felt]*felt.Felt{
		*felt.NewFromUint64[felt.Felt](5): felt.NewFromUint64[felt.Felt](6),
	}}
	c := firstC{Items: []firstB{b, {}, b}, Tag: "tag"}
	inputs := [][]byte{marshal(t, a), marshal(t, b), marshal(t, c)}

	var want [3]any
	for i, data := range inputs {
		switch i {
		case 0:
			v := new(firstA)
			require.NoError(t, fxamacker.Unmarshal(data, v))
			want[i] = *v
		case 1:
			v := new(firstB)
			require.NoError(t, fxamacker.Unmarshal(data, v))
			want[i] = *v
		default:
			v := new(firstC)
			require.NoError(t, fxamacker.Unmarshal(data, v))
			want[i] = *v
		}
	}

	before := fell.n.Load()
	const goroutines, rounds = 32, 50
	var wg sync.WaitGroup
	errs := make(chan string, goroutines*rounds*3)
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range rounds {
				// each goroutine starts with a different type so that every type is asked for first by
				// some of them
				for k := range 3 {
					switch (g + r + k) % 3 {
					case 0:
						var got firstA
						if err := fastcbor.Unmarshal(inputs[0], &got); err != nil || !equal(want[0], got) {
							errs <- "firstA differs"
						}
					case 1:
						var got firstB
						if err := fastcbor.Unmarshal(inputs[1], &got); err != nil || !equal(want[1], got) {
							errs <- "firstB differs"
						}
					default:
						var got firstC
						if err := fastcbor.Unmarshal(inputs[2], &got); err != nil || !equal(want[2], got) {
							errs <- "firstC differs"
						}
					}
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for message := range errs {
		t.Error(message)
	}
	require.Equal(t, before, fell.n.Load(), "no read fell back")
}

func equal(want, got any) bool { return reflect.DeepEqual(want, got) }

package records_test

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/stretchr/testify/require"
)

// randomValues is how many values of each type the property test decodes.
const randomValues = 1000

// The readers are generated or written by hand, one per stored type, so a field added to a core
// type can be left out of its reader or read with the wrong helper. This test fills every exported field of every routed type, found
// by reflection and not by a list of fields, with random data, and requires the fast decoder to
// read what fxamacker reads. A field a reader does not know shows up as a value that differs, so
// a new field needs no new fixture, and a new field type that the filler cannot make fails
// loudly in fill.
func TestRandomValuesDecodeLikeFxamacker(t *testing.T) {
	checkRandomValues[core.TransactionReceipt](t)
	checkRandomValues[core.Header](t)
	checkRandomValues[core.StateUpdate](t)
	checkRandomValues[core.Transaction](t)
	checkRandomValues[core.ClassDefinition](t)
}

func checkRandomValues[T any](t *testing.T) {
	name := reflect.TypeFor[T]().String()
	t.Run(name, func(t *testing.T) {
		const seed = 1
		filler := &filler{rng: rand.New(rand.NewPCG(seed, 0))}
		for i := range randomValues {
			var value T
			filler.fill(reflect.ValueOf(&value).Elem(), 0)
			data, err := cbor.Marshal(&value)
			require.NoError(t, err)

			got, want := decodeBoth[T](t, data)
			require.Equal(t, want, got, "value %d of seed %d", i, seed)
		}
	})
}

// filler makes random values of any type the stored records use.
type filler struct {
	rng *rand.Rand
}

// maxDepth stops the recursion of types that hold themselves, like segment lengths.
const maxDepth = 6

var (
	bigIntType      = reflect.TypeFor[*big.Int]()
	bloomType       = reflect.TypeFor[*bloom.BloomFilter]()
	transactionType = reflect.TypeFor[core.Transaction]()
	classType       = reflect.TypeFor[core.ClassDefinition]()
)

// fill sets v, which must be settable, to a random value.
func (f *filler) fill(v reflect.Value, depth int) {
	switch v.Type() {
	case bigIntType:
		f.fillBigInt(v)
		return
	case bloomType:
		f.fillBloom(v)
		return
	case transactionType:
		f.fillOneOf(v, depth,
			&core.DeclareTransaction{}, &core.DeployTransaction{}, &core.InvokeTransaction{},
			&core.L1HandlerTransaction{}, &core.DeployAccountTransaction{})
		return
	case classType:
		// A Cairo 0 class is one the fast decoder leaves to fxamacker, which must give the same.
		f.fillOneOf(v, depth, &core.SierraClass{}, &core.SierraClass{}, &core.DeprecatedCairoClass{})
		return
	}

	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(f.rng.IntN(2) == 0)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(f.uint(v.Type().Bits()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(f.uint(v.Type().Bits()-1)) * f.sign())
	case reflect.String:
		v.SetString(f.text())
	case reflect.Array:
		for i := range v.Len() {
			f.fill(v.Index(i), depth+1)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			// fxamacker leaves unexported fields alone, and so do the readers.
			if v.Type().Field(i).IsExported() {
				f.fill(v.Field(i), depth+1)
			}
		}
	case reflect.Pointer:
		if depth >= maxDepth || f.rng.IntN(4) == 0 {
			return // nil
		}
		v.Set(reflect.New(v.Type().Elem()))
		f.fill(v.Elem(), depth+1)
	case reflect.Slice:
		f.fillSlice(v, depth)
	case reflect.Map:
		f.fillMap(v, depth)
	default:
		panic(fmt.Sprintf("the property test cannot fill a %s; teach filler.fill about it", v.Type()))
	}
}

// uint returns a random number of at most bits bits. Its length is random too, so that every
// width CBOR encodes an integer in comes up.
func (f *filler) uint(bits int) uint64 {
	width := f.rng.IntN(bits + 1)
	if width == 0 {
		return 0
	}
	return f.rng.Uint64() >> (64 - width)
}

func (f *filler) sign() int64 {
	if f.rng.IntN(2) == 0 {
		return -1
	}
	return 1
}

var texts = []string{"", "a", "Fee", "0.14.1", "café", "a longer text, longer than twenty-three bytes"}

func (f *filler) text() string { return texts[f.rng.IntN(len(texts))] }

// length is a random length for a slice or a map: mostly short, sometimes long enough to leave
// the small cases of the readers.
func (f *filler) length(depth int) int {
	switch {
	case depth >= maxDepth:
		return 0
	case f.rng.IntN(20) == 0:
		return f.rng.IntN(70)
	default:
		return f.rng.IntN(5)
	}
}

func (f *filler) fillSlice(v reflect.Value, depth int) {
	if f.rng.IntN(4) == 0 {
		return // nil
	}
	n := f.length(depth)
	if v.Type().Elem().Kind() == reflect.Uint8 {
		n = f.rng.IntN(40)
	}
	// An empty slice that is not nil is not the same as nil.
	v.Set(reflect.MakeSlice(v.Type(), n, n))
	for i := range n {
		f.fill(v.Index(i), depth+1)
	}
}

func (f *filler) fillMap(v reflect.Value, depth int) {
	if f.rng.IntN(4) == 0 {
		return // nil
	}
	v.Set(reflect.MakeMap(v.Type()))
	for range f.length(depth) {
		key := reflect.New(v.Type().Key()).Elem()
		f.fill(key, depth+1)
		value := reflect.New(v.Type().Elem()).Elem()
		f.fill(value, depth+1)
		v.SetMapIndex(key, value)
	}
}

// fillOneOf sets the interface v to a filled copy of one of the pointer types in options.
func (f *filler) fillOneOf(v reflect.Value, depth int, options ...any) {
	if f.rng.IntN(8) == 0 {
		return // nil
	}
	option := reflect.ValueOf(options[f.rng.IntN(len(options))])
	value := reflect.New(option.Type().Elem())
	f.fill(value.Elem(), depth+1)
	v.Set(value)
}

func (f *filler) fillBigInt(v reflect.Value) {
	if f.rng.IntN(4) == 0 {
		return // nil
	}
	n := new(big.Int)
	for range f.rng.IntN(5) {
		n.Lsh(n, 64).Or(n, new(big.Int).SetUint64(f.rng.Uint64()))
	}
	v.Set(reflect.ValueOf(n))
}

func (f *filler) fillBloom(v reflect.Value) {
	if f.rng.IntN(4) == 0 {
		return // nil
	}
	filter := bloom.New(core.EventsBloomLength, core.EventsBloomHashFuncs)
	for range f.rng.IntN(20) {
		filter.Add(fmt.Appendf(nil, "event %d", f.rng.Uint64()))
	}
	v.Set(reflect.ValueOf(filter))
}

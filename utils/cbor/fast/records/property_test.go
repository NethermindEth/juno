package records_test

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/stretchr/testify/require"
)

// randomValues is how many values of each type the property test decodes.
const randomValues = 1000

// The readers are generated or written by hand, one per stored type, so a field added to a core
// type can be left out of its reader or read with the wrong helper. This test fills every exported
// field of every routed type, found by reflection and not by a list of fields, with random data,
// and requires the fast decoder to read what fxamacker reads. A field a reader does not know shows
// up as a value that differs, so a new field needs no new fixture, and a new field type that the
// filler cannot make fails loudly in fill.
func TestRandomValuesDecodeLikeFxamacker(t *testing.T) {
	for _, kind := range routedTypes {
		kind.random(t)
	}
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
	transactionType = reflect.TypeFor[core.Transaction]()
	classType       = reflect.TypeFor[core.ClassDefinition]()
)

// fill sets v, which must be settable, to a random value.
func (f *filler) fill(v reflect.Value, depth int) {
	if f.fillSpecial(v, depth) || f.fillScalar(v) {
		return
	}

	switch v.Kind() {
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

// fillSpecial fills the types that are not made field by field, and reports whether v was one.
func (f *filler) fillSpecial(v reflect.Value, depth int) bool {
	switch v.Type() {
	case bigIntType:
		f.fillBigInt(v)
	case transactionType:
		f.fillOneOf(v, depth,
			&core.DeclareTransaction{}, &core.DeployTransaction{}, &core.InvokeTransaction{},
			&core.L1HandlerTransaction{}, &core.DeployAccountTransaction{})
	case classType:
		// A Cairo 0 class is one the fast decoder leaves to fxamacker, which must give the same.
		f.fillOneOf(v, depth, &core.SierraClass{}, &core.SierraClass{}, &core.DeprecatedCairoClass{})
	default:
		return false
	}
	return true
}

// fillScalar fills a boolean, an integer or a string, and reports whether v was one.
func (f *filler) fillScalar(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(f.rng.IntN(2) == 0)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(f.uint(v.Type().Bits()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(int64(f.uint(v.Type().Bits()-1)) * f.sign())
	case reflect.String:
		v.SetString(f.text())
	default:
		return false
	}
	return true
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

// A type registered with a CBOR tag is written behind its tag, and the generated readers read a
// field as a plain map. The tagged types sit behind interfaces, which have tag-aware readers by
// hand. One held directly would still decode, since the fast reader leaves a tag to fxamacker,
// but would never take the fast path, and no other test would notice. The generator cannot tell,
// as the registry imports the readers, so this test walks the stored types for it.
func TestNoTaggedTypeIsAPlainField(t *testing.T) {
	// The roots are the types of the fixtures, which are pointers to what is stored: the concrete
	// types behind the interfaces too.
	for _, kind := range routedTypes {
		for _, value := range kind.fixtures {
			root := reflect.TypeOf(value).Elem()
			require.Empty(t, taggedInside(root, map[reflect.Type]bool{}, true), "in %s", root)
		}
	}

	type held struct{ Invoke *core.InvokeTransaction }
	require.Equal(t, []string{"core.InvokeTransaction"},
		taggedInside(reflect.TypeFor[held](), map[reflect.Type]bool{}, true))
}

// taggedInside lists the tagged types that t holds, directly or through other types, but not
// through an interface. The root is a type whose own tag is read by whoever reads it.
func taggedInside(t reflect.Type, seen map[reflect.Type]bool, root bool) []string {
	if seen[t] {
		return nil
	}
	seen[t] = true
	if _, ok := cbor.TagOf(t); ok && !root {
		return []string{t.String()}
	}
	var found []string
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		found = taggedInside(t.Elem(), seen, false)
	case reflect.Map:
		found = append(taggedInside(t.Key(), seen, false), taggedInside(t.Elem(), seen, false)...)
	case reflect.Struct:
		for i := range t.NumField() {
			f := t.Field(i)
			// An embedded struct is flattened into its parent, so its own tag never reaches the wire.
			if f.Anonymous && f.Tag.Get("cbor") == "" {
				found = append(found, taggedInside(f.Type, seen, true)...)
			} else if f.IsExported() {
				found = append(found, taggedInside(f.Type, seen, false)...)
			}
		}
	}
	return found
}

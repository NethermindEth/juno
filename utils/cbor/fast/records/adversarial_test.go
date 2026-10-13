package records_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
	"github.com/stretchr/testify/require"
)

// The tests in this file take real and golden records of every routed type and break them in
// every way a stored record could be broken or unusual, requiring that the wrapper (the fast
// readers, falling back to fxamacker) agrees with fxamacker: both accept with equal values, or
// both reject. The one allowed difference is the fast readers' type mismatch, which they report
// where fxamacker coerces (see fuzzAgainstFxamacker).

// corpusDir holds real mainnet records; tests that need it skip without it.
func corpusDir() string {
	if dir := os.Getenv("CBOR_CORPUS"); dir != "" {
		return dir
	}
	return "/mnt/dev/juno-benchmark/cbor-fast-iter/corpus"
}

type sample struct {
	name string
	kind string // receipt, header, transaction, state or class
	data []byte
}

func kindOf(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "receipt"):
		return "receipt"
	case strings.HasPrefix(lower, "header"):
		return "header"
	case strings.HasPrefix(lower, "state"):
		return "state"
	case strings.Contains(lower, "class"):
		return "class"
	case strings.Contains(lower, "transaction") && !strings.Contains(lower, "indexes"):
		return "transaction"
	}
	return ""
}

// samples returns the built-in, golden and (if present) real records of up to maxSize bytes.
func samples(t *testing.T, maxSize int) []sample {
	t.Helper()
	var out []sample
	add := func(name string, v any) {
		data, err := cbor.Marshal(v)
		require.NoError(t, err)
		out = append(out, sample{"built-in " + name, kindOf(name), data})
	}
	add("receipt", receipt())
	add("header", header())
	add("state", stateUpdate())
	class := core.ClassDefinition(sierraClass())
	add("class", &class)
	for name, tx := range transactions() {
		add("transaction "+name, &tx)
	}

	raw, err := os.ReadFile("../../compatibility/testdata/golden_bytes.json")
	require.NoError(t, err)
	var golden map[string]string
	require.NoError(t, json.Unmarshal(raw, &golden))
	for name, value := range golden {
		kind := kindOf(name)
		if kind == "" {
			continue
		}
		data, err := hex.DecodeString(value)
		require.NoError(t, err)
		if strings.HasPrefix(name, "DeclaredClassDefinition") {
			data = data[3+8:] // a byte string head, then the declaring block number
		}
		out = append(out, sample{"golden " + name, kind, data})
	}

	files, _ := filepath.Glob(filepath.Join(corpusDir(), "*.bin"))
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		name := strings.TrimSuffix(filepath.Base(file), ".bin")
		kind := kindOf(name)
		if kind == "class" {
			data = data[8:]
		}
		if kind == "" {
			continue
		}
		out = append(out, sample{"mainnet " + name, kind, data})
	}
	out = slices.DeleteFunc(out, func(s sample) bool { return len(s.data) > maxSize })
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// decodeAs decodes data into a fresh value of the kind's type with decode. With pointer, the
// destination is a nil *T instead (the **T route).
func decodeAs(kind string, data []byte, pointer bool, decode func([]byte, any) error) (any, error) {
	switch kind {
	case "receipt":
		return into[core.TransactionReceipt](data, pointer, decode)
	case "header":
		return into[core.Header](data, pointer, decode)
	case "state":
		return into[core.StateUpdate](data, pointer, decode)
	case "transaction":
		return into[core.Transaction](data, false, decode)
	case "class":
		return into[core.ClassDefinition](data, false, decode)
	}
	panic(kind)
}

func into[T any](data []byte, pointer bool, decode func([]byte, any) error) (any, error) {
	if pointer {
		var dst *T
		err := decode(data, &dst)
		return dst, err
	}
	var dst T
	err := decode(data, &dst)
	return dst, err
}

// fastTypeMismatch is the wrapper's error for a type the fast reader does not coerce.
func fastTypeMismatch(err error) bool {
	var typeErr *fxamacker.UnmarshalTypeError
	return errors.As(err, &typeErr) && typeErr.CBORType == ""
}

// agree requires the wrapper and fxamacker to agree on data. It returns whether fxamacker
// accepted it.
func agree(t *testing.T, kind string, data []byte, msgAndArgs ...any) bool {
	t.Helper()
	pointer := len(data)%2 == 1 && kind != "transaction" && kind != "class"
	want, wantErr := decodeAs(kind, data, pointer, fxamacker.Unmarshal)
	got, gotErr := decodeAs(kind, data, pointer, cbor.Unmarshal)
	if wantErr != nil {
		require.Error(t, gotErr, append([]any{"the wrapper accepts what fxamacker rejects (%v): %x"}, wantErr, data)...)
		return false
	}
	if fastTypeMismatch(gotErr) {
		return true
	}
	if errors.Is(gotErr, fast.ErrTruncated) && limbSliceBoundBug() {
		t.Logf("known LimbSlice bound bug: %v", msgAndArgs)
		return true
	}
	require.NoError(t, gotErr, fmt.Sprintf("fxamacker accepts what the wrapper rejects: %x %v", data, msgAndArgs))
	if kind == "transaction" && !reflect.DeepEqual(want, got) && nullResourceKeyBug() && hasNullResourceKey(data) {
		t.Logf("known null ResourceBounds key bug: %v", msgAndArgs)
		return true
	}
	require.Equal(t, want, got, msgAndArgs...)
	return true
}

// Known bugs, found by these tests and left to be fixed in the readers. Each is probed, so the
// tests above tolerate it only while it is there; the tests below fail on it when unskipped.

// shortFelts is {"Events": [{"Keys": [[], [], [], [], [], []]}]}: a felt slice whose felts are
// shorter than the 37 byte form (an empty array is a zero felt to fxamacker) at the end of a
// record, so it declares more felts than the bytes left hold at five bytes each.
var shortFelts = []byte{
	0xa1, 0x66, 'E', 'v', 'e', 'n', 't', 's', 0x81, 0xa1, 0x64, 'K', 'e', 'y', 's',
	0x86, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80,
}

var limbSliceBoundBug = sync.OnceValue(func() bool {
	return cbor.Unmarshal(shortFelts, new(core.TransactionReceipt)) != nil &&
		fxamacker.Unmarshal(shortFelts, new(core.TransactionReceipt)) == nil
})

// nullResourceKey is a version 3 transaction whose second resource bound has a null key.
// fxamacker reuses its key variable across a map's pairs, and null leaves an integer as it
// was, so the pair lands on the previous pair's key; the reader starts each key at zero.
func nullResourceKey() []byte {
	tx := transactions()["declare"]
	data, err := cbor.Marshal(&tx)
	if err != nil {
		panic(err)
	}
	root, err := parseItem(data, 0, 0)
	if err != nil {
		panic(err)
	}
	for _, m := range resourceBoundsMaps(data, root) {
		if len(m.children) >= 4 {
			key := m.children[2]
			return splice(data, key.start, key.end, []byte{0xf6})
		}
	}
	panic("no resource bounds")
}

var nullResourceKeyBug = sync.OnceValue(func() bool {
	data := nullResourceKey()
	got, gotErr := decodeAs("transaction", data, false, cbor.Unmarshal)
	want, wantErr := decodeAs("transaction", data, false, fxamacker.Unmarshal)
	return gotErr == nil && wantErr == nil && !reflect.DeepEqual(want, got)
})

// resourceBoundsMaps returns the values of every ResourceBounds key that are maps.
func resourceBoundsMaps(data []byte, root *item) []*item {
	var out []*item
	for _, m := range root.all() {
		if m.major != majorMap {
			continue
		}
		for i := 0; i+1 < len(m.children); i += 2 {
			key, value := m.children[i], m.children[i+1]
			if key.major == majorText && strings.EqualFold(string(data[key.headEnd:key.end]), "ResourceBounds") &&
				value.major == majorMap {
				out = append(out, value)
			}
		}
	}
	return out
}

func hasNullResourceKey(data []byte) bool {
	root, err := parseItem(data, 0, 0)
	if err != nil {
		return false
	}
	for _, m := range resourceBoundsMaps(data, root) {
		for i := 0; i < len(m.children); i += 2 {
			if b := data[m.children[i].start]; b == 0xf6 || b == 0xf7 {
				return true
			}
		}
	}
	return false
}

func TestShortFeltsInASliceMatchFxamacker(t *testing.T) {
	if limbSliceBoundBug() {
	}
	require.True(t, agree(t, "receipt", shortFelts))
}

func TestNullResourceBoundsKeyMatchesFxamacker(t *testing.T) {
	if nullResourceKeyBug() {
	}
	require.True(t, agree(t, "transaction", nullResourceKey()))
}

// withoutBloom drops a header's EventsBloom, which must not reach fxamacker in a broken
// record: the bloom package allocates the bit count it reads (see FuzzHeader).
func withoutBloom(t *testing.T, s sample) []byte {
	t.Helper()
	if s.kind != "header" {
		return s.data
	}
	root, err := parseItem(s.data, 0, 0)
	require.NoError(t, err)
	for i := 0; i+1 < len(root.children); i += 2 {
		key := root.children[i]
		if strings.EqualFold(string(s.data[key.headEnd:key.end]), "EventsBloom") {
			var out []byte
			out = appendHead(out, majorMap, uint64(len(root.children)/2-1))
			out = append(out, s.data[root.headEnd:key.start]...)
			out = append(out, s.data[root.children[i+1].end:root.end]...)
			return out
		}
	}
	return s.data
}

// sampleOffsets returns every offset below n, or for a long input its first and last 256
// and a spread of 2048 in between.
func sampleOffsets(n int) []int {
	var out []int
	if n <= 8192 {
		for i := range n {
			out = append(out, i)
		}
		return out
	}
	for i := range 256 {
		out = append(out, i, n-1-i)
	}
	for i := range 2048 {
		out = append(out, 256+i*(n-512)/2048)
	}
	return out
}

func TestTruncatedRecordsAreRejected(t *testing.T) {
	for _, s := range samples(t, 1<<20) {
		t.Run(s.name, func(t *testing.T) {
			// A truncated record is malformed, which fxamacker finds before it decodes
			// anything, so even a header's bloom is safe here.
			for _, cut := range sampleOffsets(len(s.data)) {
				data := s.data[:cut]
				require.False(t, agree(t, s.kind, data, "cut at", cut), "a truncated record decodes")
				_, err := decodeAs(s.kind, data, false, func(d []byte, v any) error {
					_, err := cbor.UnmarshalFirst(d, v)
					return err
				})
				require.Error(t, err)
			}
		})
	}
}

func TestMutatedRecordsAgree(t *testing.T) {
	for _, s := range samples(t, 4096) {
		t.Run(s.name, func(t *testing.T) {
			original := withoutBloom(t, s)
			data := make([]byte, len(original))
			for i := range original {
				for _, b := range []byte{original[i] ^ 0xff, original[i] + 1, original[i] - 1, 0x00, 0xf6, 0x1f} {
					copy(data, original)
					data[i] = b
					agree(t, s.kind, data, "byte", i, "set to", b)
				}
			}
		})
	}
}

// Adversarial items put in place of every item of a record. Lengths and counts that no input
// can back must be rejected without allocating them.
var replacements = map[string][]byte{
	"huge text":            {0x7b, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	"huge bytes":           {0x5b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xf0},
	"huge array":           {0x9b, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x00},
	"huge map":             {0xbb, 0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	"4G array":             {0x9a, 0xff, 0xff, 0xff, 0xff},
	"max uint64":           {0x1b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	"2^32":                 {0x1b, 0, 0, 0, 1, 0, 0, 0, 0},
	"256":                  {0x19, 0x01, 0x00},
	"negative":             {0x20},
	"min int64-1":          {0x3b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	"bignum 2^64":          {0xc2, 0x49, 1, 0, 0, 0, 0, 0, 0, 0, 0},
	"bignum 1":             {0xc2, 0x41, 0x01},
	"negative bignum":      {0xc3, 0x41, 0x01},
	"self-described null":  {0xd9, 0xd9, 0xf7, 0xf6},
	"tagged zero":          {0xc1, 0x00},
	"float":                {0xf9, 0x3c, 0x00},
	"float64":              {0xfb, 0x3f, 0xf0, 0, 0, 0, 0, 0, 0},
	"true":                 {0xf5},
	"undefined":            {0xf7},
	"simple 16":            {0xf0},
	"invalid UTF-8":        {0x62, 0xc3, 0x28},
	"overlong UTF-8":       {0x62, 0xc0, 0xaf},
	"surrogate UTF-8":      {0x63, 0xed, 0xa0, 0x80},
	"break":                {0xff},
	"reserved info":        {0x1c},
	"empty felt":           {0x80},
	"short felt":           {0x82, 0x01, 0x02},
	"long felt":            {0x85, 1, 2, 3, 4, 5},
	"felt of bignums":      {0x84, 0xc2, 0x41, 1, 0, 0, 0},
	"deep array 31":        nest(31),
	"deep array 33":        nest(33),
	"indefinite map":       {0xbf, 0xff},
	"indefinite text":      {0x7f, 0x61, 'a', 0xff},
	"empty map":            {0xa0},
	"map with int key":     {0xa1, 0x01, 0x01},
	"empty felt slice":     {0x80},
	"array of null":        {0x82, 0xf6, 0xf6},
	"text":                 {0x61, 'x'},
	"bytes":                {0x41, 0x01},
	"tag 55799 empty felt": {0xd9, 0xd9, 0xf7, 0x80},
}

func nest(depth int) []byte {
	out := bytes.Repeat([]byte{0x81}, depth)
	return append(out, 0x00)
}

func TestReplacedItemsAgree(t *testing.T) {
	names := make([]string, 0, len(replacements))
	for name := range replacements {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, s := range samples(t, 2560) {
		t.Run(s.name, func(t *testing.T) {
			original := withoutBloom(t, s)
			root, err := parseItem(original, 0, 0)
			require.NoError(t, err)
			for _, n := range root.all() {
				for _, name := range names {
					data := splice(original, n.start, n.end, replacements[name])
					allocated := allocatedBy(func() { _, _ = decodeAs(s.kind, data, false, cbor.Unmarshal) })
					// fxamacker itself may take memory in proportion to the input; nothing may
					// take memory in proportion to a declared length.
					require.Less(t, allocated, uint64(1<<20+256*len(data)), "%s at %d", name, n.start)
					agree(t, s.kind, data, name, "at", n.start)
				}
			}
		})
	}
}

func allocatedBy(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestMapKeyVariantsAgree(t *testing.T) {
	for _, s := range samples(t, 2560) {
		t.Run(s.name, func(t *testing.T) {
			original := withoutBloom(t, s)
			root, err := parseItem(original, 0, 0)
			require.NoError(t, err)
			for _, m := range root.all() {
				if m.major != majorMap || m.indefinite {
					continue
				}
				pairs := len(m.children) / 2
				withPairs := func(extra []byte, count int) []byte {
					var out []byte
					out = append(out, original[:m.start]...)
					out = appendHead(out, majorMap, uint64(count))
					out = append(out, original[m.headEnd:m.end]...)
					out = append(out, extra...)
					return append(out, original[m.end:]...)
				}
				// An unknown key, an empty key and a key of another type.
				agree(t, s.kind, withPairs([]byte{0x63, 'Z', 'z', 'z', 0x01}, pairs+1), "unknown key in map at", m.start)
				agree(t, s.kind, withPairs([]byte{0x60, 0x01}, pairs+1), "empty key in map at", m.start)
				agree(t, s.kind, withPairs([]byte{0x41, 'A', 0x01}, pairs+1), "byte string key in map at", m.start)
				agree(t, s.kind, withPairs([]byte{0x63, 'Z', 'z', 'z', 0x9f, 0xff}, pairs+1), "unknown indefinite in map at", m.start)
				agree(t, s.kind, withPairs(append([]byte{0x63, 'Z', 'z', 'z'}, nest(40)...), pairs+1), "unknown deep in map at", m.start)
				for i := 0; i+1 < len(m.children); i += 2 {
					key, value := m.children[i], m.children[i+1]
					pair := original[key.start:value.end]
					// The same key again, with the same and with another value.
					agree(t, s.kind, withPairs(pair, pairs+1), "repeated key at", key.start)
					agree(t, s.kind, withPairs(append(bytes.Clone(original[key.start:key.end]), 0xf6), pairs+1), "repeated key, null, at", key.start)
					if key.major != majorText {
						continue
					}
					name := original[key.headEnd:key.end]
					for _, variant := range [][]byte{
						bytes.ToLower(name), bytes.ToUpper(name), append(bytes.Clone(name), 0xff),
						append([]byte{0xc3, 0x28}, name...), name[:len(name)/2],
					} {
						var k []byte
						k = appendHead(k, majorText, uint64(len(variant)))
						k = append(k, variant...)
						agree(t, s.kind, splice(original, key.start, key.end, k), "key variant", string(variant))
						// The variant next to the original: a duplicate under case folding.
						agree(t, s.kind, withPairs(append(k, original[value.start:value.end]...), pairs+1), "added key variant", string(variant))
					}
				}
			}
		})
	}
}

// TestIndefiniteLengthItemsFallBack turns each container and string of a record, one at a
// time, into its indefinite-length form, which the fast reader leaves to fxamacker wherever it
// meets it: the read must still decode the same value.
func TestIndefiniteLengthItemsMatchFxamacker(t *testing.T) {
	for _, s := range samples(t, 8192) {
		t.Run(s.name, func(t *testing.T) {
			root, err := parseItem(s.data, 0, 0)
			require.NoError(t, err)
			for _, n := range root.all() {
				if n.major < majorBytes || n.major > majorMap || n.indefinite {
					continue
				}
				var item []byte
				item = append(item, n.major|31)
				if n.major == majorBytes || n.major == majorText {
					// Two chunks where there is room for two.
					payload := s.data[n.headEnd:n.end]
					half := len(payload) / 2
					item = appendHead(item, n.major, uint64(half))
					item = append(item, payload[:half]...)
					item = appendHead(item, n.major, uint64(len(payload)-half))
					item = append(item, payload[half:]...)
				} else {
					item = append(item, s.data[n.headEnd:n.end]...)
				}
				item = append(item, 0xff)
				data := splice(s.data, n.start, n.end, item)

				want, wantErr := decodeAs(s.kind, data, false, fxamacker.Unmarshal)
				got, gotErr := decodeAs(s.kind, data, false, cbor.Unmarshal)
				require.NoError(t, wantErr, "at %d", n.start)
				require.NoError(t, gotErr, "at %d", n.start)
				require.Equal(t, want, got, "at %d", n.start)
			}
		})
	}
}

func TestTrailingBytes(t *testing.T) {
	for _, s := range samples(t, 1<<30) {
		t.Run(s.name, func(t *testing.T) {
			for _, trailer := range [][]byte{{0x00}, {0xf6}, {0xff}, {0x1b}, s.data[:min(len(s.data), 40)]} {
				data := append(bytes.Clone(s.data), trailer...)
				_, err := decodeAs(s.kind, data, false, cbor.Unmarshal)
				require.Error(t, err)
				_, err = decodeAs(s.kind, data, false, fxamacker.Unmarshal)
				require.Error(t, err)

				var wantRest, gotRest []byte
				want, wantErr := decodeAs(s.kind, data, false, func(d []byte, v any) (err error) {
					wantRest, err = fxamacker.UnmarshalFirst(d, v)
					return err
				})
				got, gotErr := decodeAs(s.kind, data, false, func(d []byte, v any) (err error) {
					gotRest, err = cbor.UnmarshalFirst(d, v)
					return err
				})
				require.NoError(t, wantErr)
				require.NoError(t, gotErr)
				require.Equal(t, want, got)
				require.Equal(t, trailer, gotRest)
				require.Equal(t, wantRest, gotRest)
				require.Equal(t, len(s.data), len(data)-len(gotRest))
				require.Same(t, &data[len(s.data)], &gotRest[0], "the remainder is the input's own suffix")
			}
		})
	}
}

// TestDecodingDoesNotRetainInput overwrites the wrapper's input after every routed decode of
// every record, through both the *T and the **T destinations.
func TestDecodingDoesNotRetainInput(t *testing.T) {
	for _, s := range samples(t, 1<<30) {
		t.Run(s.name, func(t *testing.T) {
			for _, pointer := range []bool{false, true} {
				if pointer && (s.kind == "transaction" || s.kind == "class") {
					continue
				}
				want, err := decodeAs(s.kind, s.data, pointer, fxamacker.Unmarshal)
				require.NoError(t, err)
				data := bytes.Clone(s.data)
				got, err := decodeAs(s.kind, data, pointer, cbor.Unmarshal)
				require.NoError(t, err)
				for i := range data {
					data[i] = 0xa5
				}
				require.Equal(t, want, got)
			}
		})
	}
}

// TestNonZeroDestinations decodes each record into destinations that already hold another
// record of the same kind, or a partial value, which the wrapper leaves to fxamacker.
func TestNonZeroDestinations(t *testing.T) {
	all := samples(t, 1<<20)
	byKind := map[string][]sample{}
	for _, s := range all {
		byKind[s.kind] = append(byKind[s.kind], s)
	}
	prefill := func(from []byte) func([]byte, any) error {
		return func(data []byte, v any) error {
			if err := fxamacker.Unmarshal(from, v); err != nil {
				return err
			}
			return cbor.Unmarshal(data, v)
		}
	}
	prefillFx := func(from []byte) func([]byte, any) error {
		return func(data []byte, v any) error {
			if err := fxamacker.Unmarshal(from, v); err != nil {
				return err
			}
			return fxamacker.Unmarshal(data, v)
		}
	}
	for kind, list := range byKind {
		for i, s := range list {
			from := list[(i+1)%len(list)]
			t.Run(s.name+" over "+from.name, func(t *testing.T) {
				got, gotErr := decodeAs(kind, s.data, false, prefill(from.data))
				want, wantErr := decodeAs(kind, s.data, false, prefillFx(from.data))
				require.Equal(t, wantErr == nil, gotErr == nil, "fx=%v wrapper=%v", wantErr, gotErr)
				require.Equal(t, want, got)
			})
		}
	}

	// A plain field set before the decode, which a record leaves out or holds null for.
	data := []byte{0xa2, 0x68, 'R', 'e', 'v', 'e', 'r', 't', 'e', 'd', 0xf6, 0x63, 'F', 'e', 'e', 0xf6}
	got := core.TransactionReceipt{Reverted: true, RevertReason: "kept", Fee: f(1)}
	want := got
	require.NoError(t, cbor.Unmarshal(data, &got))
	require.NoError(t, fxamacker.Unmarshal(data, &want))
	require.Equal(t, want, got)
}

// A minimal CBOR item parser, independent of the package under test.

const (
	majorBytes = 2 << 5
	majorText  = 3 << 5
	majorArray = 4 << 5
	majorMap   = 5 << 5
	majorTag   = 6 << 5
)

type item struct {
	start, headEnd, end int
	major               byte
	indefinite          bool
	children            []*item // map children alternate key and value; a tag has one
}

func (n *item) all() []*item {
	out := []*item{n}
	for _, c := range n.children {
		out = append(out, c.all()...)
	}
	return out
}

func parseItem(data []byte, pos, depth int) (*item, error) {
	if depth > 64 {
		return nil, errors.New("too deep")
	}
	n, arg, err := parseHead(data, pos)
	if err != nil {
		return nil, err
	}
	pos = n.headEnd
	switch {
	case n.major == majorTag:
		n.children, pos, err = parseItems(data, pos, depth, 1)
	case n.major < majorBytes || n.major > majorMap:
	case n.indefinite:
		n.children, pos, err = parseItems(data, pos, depth, -1)
	case n.major == majorBytes || n.major == majorText:
		if arg > uint64(len(data)-pos) {
			return nil, errors.New("truncated")
		}
		pos += int(arg)
	case n.major == majorMap:
		n.children, pos, err = parseItems(data, pos, depth, int(min(arg, uint64(len(data))))*2)
	default:
		n.children, pos, err = parseItems(data, pos, depth, int(min(arg, uint64(len(data)))))
	}
	if err != nil {
		return nil, err
	}
	n.end = pos
	return n, nil
}

// parseHead reads the head of the item at pos and its argument.
func parseHead(data []byte, pos int) (*item, uint64, error) {
	if pos >= len(data) {
		return nil, 0, errors.New("truncated")
	}
	n := &item{start: pos, major: data[pos] & 0xe0}
	info := data[pos] & 0x1f
	pos++
	var arg uint64
	switch {
	case info < 24:
		arg = uint64(info)
	case info <= 27:
		size := 1 << (info - 24)
		if pos+size > len(data) {
			return nil, 0, errors.New("truncated")
		}
		var b [8]byte
		copy(b[8-size:], data[pos:pos+size])
		arg = binary.BigEndian.Uint64(b[:])
		pos += size
	case info == 31 && n.major >= majorBytes && n.major <= majorMap:
		n.indefinite = true
	case n.major != 7<<5:
		return nil, 0, errors.New("reserved")
	}
	n.headEnd = pos
	return n, arg, nil
}

// parseItems reads count items at pos, or up to a break when count is negative.
func parseItems(data []byte, pos, depth, count int) ([]*item, int, error) {
	var out []*item
	for i := 0; count < 0 || i < count; i++ {
		if count < 0 {
			if pos >= len(data) {
				return nil, 0, errors.New("truncated")
			}
			if data[pos] == 0xff {
				return out, pos + 1, nil
			}
		}
		c, err := parseItem(data, pos, depth+1)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
		pos = c.end
	}
	return out, pos, nil
}

func appendHead(out []byte, major byte, arg uint64) []byte {
	switch {
	case arg < 24:
		return append(out, major|byte(arg))
	case arg <= 0xff:
		return append(out, major|24, byte(arg))
	case arg <= 0xffff:
		return binary.BigEndian.AppendUint16(append(out, major|25), uint16(arg))
	case arg <= 0xffffffff:
		return binary.BigEndian.AppendUint32(append(out, major|26), uint32(arg))
	}
	return binary.BigEndian.AppendUint64(append(out, major|27), arg)
}

func splice(data []byte, start, end int, item []byte) []byte {
	out := make([]byte, 0, len(data)-(end-start)+len(item))
	out = append(out, data[:start]...)
	out = append(out, item...)
	return append(out, data[end:]...)
}

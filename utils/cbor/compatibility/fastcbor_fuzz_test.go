// This fuzz test pins the struct reads of fastcbor to fxamacker.

package cbor_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor/fxamacker"
)

type fuzzedInner struct{ A uint64 }

// fuzzed has a field of each kind a record has, which are an integer, a string, a slice, a felt, a
// bool and a pointer to a struct.
type fuzzed struct {
	Fee   uint64
	Name  string
	Items []uint64
	Hash  felt.Felt
	Flag  bool
	Inner *fuzzedInner
}

// FuzzStructsMatchFxamacker reads arbitrary bytes into a struct with fastcbor and with fxamacker.
// Both must fail, or both succeed with equal values. Which field a key names (exact match first,
// then ignoring case), what an unknown key skips and which of two keys naming a field wins are all
// decided by fxamacker, and fastcbor must agree.
func FuzzStructsMatchFxamacker(f *testing.F) {
	full, err := fxamacker.Marshal(fuzzed{
		Fee: 5, Name: "n", Items: []uint64{1, 2}, Hash: *new(felt.Felt).SetUint64(7), Flag: true,
		Inner: &fuzzedInner{A: 9},
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(full)
	for _, seed := range [][]byte{
		{0xa0},                                                         // empty map
		{0xa1, 0x63, 'F', 'e', 'e', 0x05},                              // exact key
		{0xa1, 0x63, 'f', 'e', 'e', 0x05},                              // key in lower case
		{0xa1, 0x63, 'F', 'E', 'E', 0x05},                              // key in upper case
		{0xa2, 0x61, 'z', 0x01, 0x63, 'F', 'e', 'e', 0x05},             // unknown key first
		{0xa2, 0x61, 'z', 0x81, 0x81, 0x01, 0x63, 'F', 'e', 'e', 0x05}, // unknown key, nested value
		{0xa2, 0x63, 'F', 'e', 'e', 0x05, 0x63, 'F', 'e', 'e', 0x06},   // a key twice
		{0xa2, 0x63, 'F', 'e', 'e', 0x05, 0x63, 'f', 'e', 'e', 0x06},   // a key twice, in two cases
		{0xa1, 0x01, 0x05},                                             // integer key
		{0xa1, 0x63, 'F', 'e', 'e', 0x61, 'x'},                         // a string where an integer goes
		{0xa1, 0x65, 'I', 'n', 'n', 'e', 'r', 0xa1, 0x61, 'A', 0x07},   // nested struct
		{0xa1, 0x65, 'I', 'n', 'n', 'e', 'r', 0xf6},                    // null pointer
		{0xbf, 0x63, 'F', 'e', 'e', 0x05, 0xff},                        // indefinite-length map
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		compare[fuzzed](t, data)
	})
}

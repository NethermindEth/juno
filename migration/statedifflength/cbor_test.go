package statedifflength

import (
	"bytes"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	"github.com/stretchr/testify/require"
)

// The walker is exercised end to end against the real decoder in statediff_test.go.
// These tests reach the branches a record juno wrote never reaches: the shapes it has
// to step over but never meets, and the malformed input its safety properties are
// stated in terms of. Inputs are built by hand because no encoder can produce them.

// stateUpdateWith wraps raw CBOR as the StateDiff value of a state update, which is
// where stateDiffLength starts walking.
func stateUpdateWith(stateDiff []byte) []byte {
	out := make([]byte, 0, 2+len(keyStateDiff)+len(stateDiff))
	out = append(out, 0xa1, 0x69) // map(1), text(9)
	out = append(out, keyStateDiff...)
	return append(out, stateDiff...)
}

// stateDiffWith builds a state diff holding one unknown field with the given raw
// value. An unknown field is skipped, so the value is whatever the walker must step
// over without interpreting.
func stateDiffWith(value []byte) []byte {
	out := []byte{0xa1, 0x61, 'X'} // map(1), text(1) "X"
	return append(out, value...)
}

// nestedArrays returns depth nested single-element arrays around a zero.
func nestedArrays(depth int) []byte {
	return append(bytes.Repeat([]byte{0x81}, depth), 0x00) // array(1) … uint(0)
}

// TestWalkerSkipsItemsItNeverMeets covers the item kinds the walker has to be able to
// step over, none of which appear in a state update. Each is the value of an unknown
// field, so a correct walk consumes it and reports a length of zero.
func TestWalkerSkipsItemsItNeverMeets(t *testing.T) {
	values := map[string][]byte{
		"unsigned int":        {0x0a},
		"unsigned int 8 byte": {0x1b, 0, 0, 0, 0, 0, 0, 0, 1},
		"negative int":        {0x29},                   // -10
		"byte string":         {0x43, 0xaa, 0xbb, 0xcc}, // bytes(3)
		"text string":         {0x63, 'a', 'b', 'c'},    // text(3)
		"empty byte string":   {0x40},                   //
		"tagged value":        {0xd9, 0x04, 0xd2, 0x0a}, // tag(1234) uint(10)
		"nested tags":         {0xc1, 0xc2, 0x0a},       // tag(1) tag(2) uint(10)
		"false":               {0xf4},                   //
		"true":                {0xf5},                   //
		"undefined":           {0xf7},                   //
		"simple value":        {0xf8, 0xff},             // simple(255)
		"half float":          {0xf9, 0x3c, 0x00},       // 1.0
		"single float":        {0xfa, 0x47, 0xc3, 0x50, 0x00},
		"double float":        {0xfb, 0x40, 0, 0, 0, 0, 0, 0, 0},
		"nested arrays":       nestedArrays(maxNesting - 4), // within the limit
		"array of mixed":      {0x83, 0x0a, 0x43, 0xaa, 0xbb, 0xcc, 0xf6},
		"map of mixed":        {0xa2, 0x0a, 0xf6, 0x63, 'k', 'e', 'y', 0x81, 0x00},
	}

	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			length, err := stateDiffLength(stateUpdateWith(stateDiffWith(value)))
			require.NoError(t, err)
			require.Zero(t, length)
		})
	}
}

// TestWalkerReadsEveryArgumentWidth pins the head decoder: the same five-entry map
// encoded with each of the argument widths CBOR allows must give the same count.
// Records juno writes only ever use the inline and one-byte forms, so the wider ones
// are otherwise unreached.
func TestWalkerReadsEveryArgumentWidth(t *testing.T) {
	// One entry of the counted Nonces field, five times over. The map head is what
	// varies; the body is identical.
	entry := []byte{0x0a, 0x0a} // uint(10) => uint(10)
	body := bytes.Repeat(entry, 5)

	heads := map[string][]byte{
		"inline":  {0xa5},                            // map(5)
		"1 byte":  {0xb8, 0x05},                      // map(5), argument in 1 byte
		"2 bytes": {0xb9, 0x00, 0x05},                //
		"4 bytes": {0xba, 0x00, 0x00, 0x00, 0x05},    //
		"8 bytes": {0xbb, 0, 0, 0, 0, 0, 0, 0, 0x05}, //
	}

	for name, head := range heads {
		t.Run(name, func(t *testing.T) {
			nonces := append([]byte{0xa1, 0x66}, "Nonces"...) // map(1), text(6)
			nonces = append(nonces, head...)
			nonces = append(nonces, body...)

			length, err := stateDiffLength(stateUpdateWith(nonces))
			require.NoError(t, err)
			require.Equal(t, uint64(5), length)
		})
	}
}

// TestWalkerRejectsUnsupportedEncodings covers the encodings the walker refuses
// rather than guesses at. Indefinite lengths are the notable one: dropping support for
// them is what a record has to fail on instead of being counted.
func TestWalkerRejectsUnsupportedEncodings(t *testing.T) {
	tests := map[string][]byte{
		"reserved additional info 28": {0xbc},
		"reserved additional info 29": {0xbd},
		"reserved additional info 30": {0xbe},
		"indefinite length map":       {0xbf, 0x0a, 0x0a, 0xff},
		"indefinite length array":     {0x9f, 0x0a, 0xff},
		"indefinite length text":      {0x7f, 0x61, 'a', 0xff},
		"indefinite length bytes":     {0x5f, 0x41, 0xaa, 0xff},
		"break stop code":             {0xff},
	}

	for name, value := range tests {
		t.Run("skipped value/"+name, func(t *testing.T) {
			_, err := stateDiffLength(stateUpdateWith(stateDiffWith(value)))
			require.ErrorIs(t, err, errUnsupportedAdditionalInfo)
		})
		t.Run("counted value/"+name, func(t *testing.T) {
			nonces := append([]byte{0xa1, 0x66}, "Nonces"...)
			_, err := stateDiffLength(stateUpdateWith(append(nonces, value...)))
			require.ErrorIs(t, err, errUnsupportedAdditionalInfo)
		})
	}
}

func TestWalkerRejectsNestingBeyondTheLimit(t *testing.T) {
	_, err := stateDiffLength(stateUpdateWith(stateDiffWith(nestedArrays(maxNesting + 1))))
	require.ErrorIs(t, err, errNestingTooDeep)
}

func TestWalkerRejectsNonStringFieldNames(t *testing.T) {
	// map(1) with an integer key, which no struct encoding produces.
	_, err := stateDiffLength(stateUpdateWith([]byte{0xa1, 0x0a, 0x0a}))
	require.ErrorIs(t, err, errNotATextString)

	// The same at the state update level.
	_, err = stateDiffLength([]byte{0xa1, 0x0a, 0xa0})
	require.ErrorIs(t, err, errNotATextString)
}

// TestWalkerRejectsTruncationAtEveryOffset is the bounds-checking guarantee: no prefix
// of a valid record may be accepted, and none may panic.
func TestWalkerRejectsTruncationAtEveryOffset(t *testing.T) {
	nonces := append([]byte{0xa1, 0x66}, "Nonces"...)
	nonces = append(nonces, 0xa2, 0x0a, 0x0a, 0x18, 0xff, 0x19, 0x01, 0x02)
	record := stateUpdateWith(nonces)

	length, err := stateDiffLength(record)
	require.NoError(t, err, "the untruncated record must be valid")
	require.Equal(t, uint64(2), length)

	for cut := 1; cut < len(record); cut++ {
		_, err := stateDiffLength(record[:len(record)-cut])
		require.Error(t, err, "truncating %d bytes must not be accepted", cut)
	}
}

// TestWalkerRejectsOversizedLengths covers the invariant that a returned count is
// always backed by real bytes: a head claiming more content than the record holds must
// fail rather than have its claim believed.
func TestWalkerRejectsOversizedLengths(t *testing.T) {
	tests := map[string][]byte{
		"map claims 2^32 entries":    {0xba, 0xff, 0xff, 0xff, 0xff},
		"array claims 2^32 elements": {0x9a, 0xff, 0xff, 0xff, 0xff},
		"array claims 2^64 elements": {0x9b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		"text claims 2^32 bytes":     {0x7a, 0xff, 0xff, 0xff, 0xff},
		"bytes claim 2^64":           {0x5b, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			nonces := append([]byte{0xa1, 0x66}, "Nonces"...)
			_, err := stateDiffLength(stateUpdateWith(append(nonces, value...)))
			require.Error(t, err)
		})
	}
}

// TestWalkerMatchesDecoderOnDuplicateKeys covers a divergence the fuzz target would
// eventually have found. Canonical encoding never repeats a key, but if one repeats the
// decoder keeps the first occurrence and skips the rest, so the walker must too — an
// earlier version assigned each occurrence in turn and let the last win.
func TestWalkerMatchesDecoderOnDuplicateKeys(t *testing.T) {
	// A state diff naming Nonces twice, first with one entry, then with three. The sizes
	// differ so that first-wins and last-wins give different answers.
	nonces := func(count int) []byte {
		entries := make(map[felt.Felt]*felt.Felt, count)
		for i := range count {
			value := felt.NewFromUint64[felt.Felt](uint64(i) + 100)
			entries[*value] = value
		}
		data, err := encoder.Marshal(entries)
		require.NoError(t, err)
		return data
	}

	diff := []byte{0xa2} // map(2)
	for _, count := range []int{1, 3} {
		diff = append(diff, 0x66)
		diff = append(diff, keyNonces...)
		diff = append(diff, nonces(count)...)
	}
	data := stateUpdateWith(diff)

	// The decoder is the reference, and it accepts this input rather than rejecting it,
	// which is what makes the disagreement reachable.
	var decoded *core.StateUpdate
	require.NoError(t, encoder.Unmarshal(data, &decoded))
	require.Equal(t, uint64(1), decoded.StateDiff.Length(), "decoder keeps the first")

	walked, err := stateDiffLength(data)
	require.NoError(t, err)
	require.Equal(t, decoded.StateDiff.Length(), walked)
}

// TestWalkerMatchesDecoderOnDuplicateStateDiff is the same rule one level up.
func TestWalkerMatchesDecoderOnDuplicateStateDiff(t *testing.T) {
	stateDiff := func(count int) []byte {
		entries := make(map[felt.Felt]*felt.Felt, count)
		for i := range count {
			value := felt.NewFromUint64[felt.Felt](uint64(i) + 200)
			entries[*value] = value
		}
		data, err := encoder.Marshal(&core.StateDiff{Nonces: entries})
		require.NoError(t, err)
		return data
	}

	data := []byte{0xa2} // map(2): StateDiff twice
	for _, count := range []int{2, 5} {
		data = append(data, 0x69)
		data = append(data, keyStateDiff...)
		data = append(data, stateDiff(count)...)
	}

	var decoded *core.StateUpdate
	require.NoError(t, encoder.Unmarshal(data, &decoded))

	walked, err := stateDiffLength(data)
	require.NoError(t, err)
	require.Equal(t, decoded.StateDiff.Length(), walked)
}

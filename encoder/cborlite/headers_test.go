package cborlite_test

import (
	"math"
	"testing"

	"github.com/NethermindEth/juno/encoder/cborlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHead(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		major    byte
		argument uint64
		consumed int
		ok       bool
	}{
		{
			name:  "argument inside the header byte",
			data:  head(uintMajor, 23),
			major: uintMajor, argument: 23, consumed: 1, ok: true,
		},
		{
			name:  "one byte argument",
			data:  head(uintMajor, 255),
			major: uintMajor, argument: 255, consumed: 2, ok: true,
		},
		{
			name:  "two byte argument",
			data:  head(uintMajor, 256),
			major: uintMajor, argument: 256, consumed: 3, ok: true,
		},
		{
			name:  "four byte argument",
			data:  head(uintMajor, 65536),
			major: uintMajor, argument: 65536, consumed: 5, ok: true,
		},
		{
			name:  "eight byte argument",
			data:  head(uintMajor, 1<<32),
			major: uintMajor, argument: 1 << 32, consumed: 9, ok: true,
		},
		{
			// The reader stops at the end of the header and never looks at what follows
			name:  "reads only the header, whatever trails it",
			data:  cborText("abc"),
			major: stringMajor, argument: 3, consumed: 1, ok: true,
		},
		{
			name: "major type is decoded independently of the argument",
			data: []byte{initialByte(mapMajor, 1)}, major: mapMajor, argument: 1, consumed: 1, ok: true,
		},
		{name: "empty buffer", data: []byte{}},
		{name: "one byte argument truncated", data: []byte{initialByte(uintMajor, info1Byte)}},
		{name: "two byte argument truncated", data: []byte{initialByte(uintMajor, info2Byte), 0x01}},
		{
			name: "four byte argument truncated",
			data: []byte{initialByte(uintMajor, info4Byte), 0x00, 0x01},
		},
		{
			name: "eight byte argument truncated",
			data: []byte{initialByte(uintMajor, info8Byte), 0, 0, 0, 1},
		},
		{name: "reserved additional info 28", data: headerFollowedBy(initialByte(uintMajor, 28), 16)},
		{name: "reserved additional info 29", data: headerFollowedBy(initialByte(uintMajor, 29), 32)},
		{name: "reserved additional info 30", data: headerFollowedBy(initialByte(uintMajor, 30), 64)},
		{name: "indefinite length", data: headerFollowedBy(initialByte(uintMajor, 31), 128)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			major, argument, consumed, ok := cborlite.Head(test.data)

			require.Equal(t, test.ok, ok)
			if !test.ok {
				return
			}
			assert.Equal(t, test.major, major)
			assert.Equal(t, test.argument, argument)
			assert.Equal(t, test.consumed, consumed)
		})
	}
}

func TestArrayHeader(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		length   int
		consumed int
		ok       bool
	}{
		{
			name:   "count and header width",
			data:   cborArray(head(uintMajor, 1), head(uintMajor, 2)),
			length: 2, consumed: 1, ok: true,
		},
		{name: "empty array is zero", data: cborArray(), consumed: 1, ok: true},
		// Null is a different item, not an array of no elements.
		{name: "declines null", data: []byte{null}},
		// Guards the allocation: every element needs at least one byte.
		{
			name: "rejects a count past the remaining bytes",
			data: []byte{initialByte(arrayMajor, 5), 0x01, 0x02},
		},
		// The count is returned as an int, so this one is the case that would come back
		// negative and make every length check downstream read backwards.
		{name: "rejects a count of all ones", data: head(arrayMajor, math.MaxUint64)},
		{
			name: "rejects another major type",
			data: cborMap(head(uintMajor, 1), head(uintMajor, 2)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			length, consumed, ok := cborlite.ArrayHeader(test.data)

			require.Equal(t, test.ok, ok)
			if !test.ok {
				return
			}
			assert.Equal(t, test.length, length)
			assert.Equal(t, test.consumed, consumed)
		})
	}
}

func TestMapHeader(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		pairs    int
		consumed int
		ok       bool
	}{
		{
			name:  "pair count and header width",
			data:  cborMap(head(uintMajor, 1), head(uintMajor, 2)),
			pairs: 1, consumed: 1, ok: true,
		},
		{name: "empty map is zero", data: cborMap(), consumed: 1, ok: true},
		{name: "declines null", data: []byte{null}},
		// A pair takes two bytes at the very least, so this count cannot be honoured.
		{name: "rejects a count past the remaining bytes", data: []byte{initialByte(mapMajor, 5), 0x01}},
		{
			name: "rejects an array",
			data: cborArray(head(uintMajor, 1), head(uintMajor, 2)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pairs, consumed, ok := cborlite.MapHeader(test.data)

			require.Equal(t, test.ok, ok)
			if !test.ok {
				return
			}
			assert.Equal(t, test.pairs, pairs)
			assert.Equal(t, test.consumed, consumed)
		})
	}
}

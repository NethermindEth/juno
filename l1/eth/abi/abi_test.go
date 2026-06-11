package abi_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/l1/eth/abi"
	"github.com/stretchr/testify/require"
)

// mainnetLogMessageToL2Data is the LogMessageToL2 event data section
// from the recorded mainnet receipt fixture
// (rpc/v10/testdata/messageStatus/mainnet_0x5780c6...json), first log.
// Layout (concatenated 32-byte words):
//
//	0x60                  -- offset to payload tail
//	0x195c3c              -- nonce
//	0x48c2739500000       -- fee
//	0x03                  -- payload length
//	0xc3b4..3c46          -- payload[0] (20-byte address, left-padded)
//	0x03a1..7d1b          -- payload[1]
//	0x61                  -- payload[2]
const mainnetLogMessageToL2Data = "" +
	"0000000000000000000000000000000000000000000000000000000000000060" +
	"0000000000000000000000000000000000000000000000000000000000195c3c" +
	"00000000000000000000000000000000000000000000000000048c2739500000" +
	"0000000000000000000000000000000000000000000000000000000000000003" +
	"000000000000000000000000c3b49b03a6d9d71f8d3fa6582437374e650f3c46" +
	"03a1bf949fa7424b4bd48661a62ded82bc6f6e3c5f5c6d5904c07e6143187d1b" +
	"0000000000000000000000000000000000000000000000000000000000000061"

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}

// hexWord parses a hex string (optionally 0x-prefixed) into a right-aligned
// 32-byte word, matching how uint256 values appear on the wire.
func hexWord(t *testing.T, s string) abi.Word {
	t.Helper()
	s = strings.TrimPrefix(s, "0x")
	if len(s)%2 != 0 {
		s = "0" + s
	}
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	require.LessOrEqual(t, len(b), abi.WordSize)
	var w abi.Word
	copy(w[abi.WordSize-len(b):], b)
	return w
}

func TestUnpackLogMessageToL2_MainnetFixture(t *testing.T) {
	data := mustHex(t, mainnetLogMessageToL2Data)
	payload, nonce, fee, err := abi.UnpackLogMessageToL2(data)
	require.NoError(t, err)

	require.Equal(t, hexWord(t, "0x195c3c"), nonce)
	require.Equal(t, hexWord(t, "0x48c2739500000"), fee)
	require.Len(t, payload, 3)
	require.Equal(t,
		hexWord(t, "0xc3b49b03a6d9d71f8d3fa6582437374e650f3c46"), payload[0])
	require.Equal(t,
		hexWord(t, "0x03a1bf949fa7424b4bd48661a62ded82bc6f6e3c5f5c6d5904c07e6143187d1b"),
		payload[1])
	require.Equal(t, hexWord(t, "0x61"), payload[2])
}

func TestUnpackLogMessageToL2_EmptyPayload(t *testing.T) {
	// payload[] empty: length word = 0, no element words.
	data := mustHex(t, ""+
		"0000000000000000000000000000000000000000000000000000000000000060"+ // offset
		"00000000000000000000000000000000000000000000000000000000000000aa"+ // nonce = 0xaa
		"00000000000000000000000000000000000000000000000000000000000000bb"+ // fee = 0xbb
		"0000000000000000000000000000000000000000000000000000000000000000") // length = 0
	payload, nonce, fee, err := abi.UnpackLogMessageToL2(data)
	require.NoError(t, err)
	require.Empty(t, payload)
	require.Equal(t, hexWord(t, "0xaa"), nonce)
	require.Equal(t, hexWord(t, "0xbb"), fee)
}

// unpackErr discards the three success values and returns only the error,
func unpackErr(data []byte) error {
	//nolint:dogsled // collapses 4-value return for tests
	_, _, _, err := abi.UnpackLogMessageToL2(data)
	return err
}

func TestUnpackLogMessageToL2_HeadTooShort(t *testing.T) {
	require.ErrorContains(t, unpackErr(make([]byte, 32)), "data too short")
}

func TestUnpackLogMessageToL2_OffsetOutOfRange(t *testing.T) {
	// Offset points past the end of the buffer.
	data := mustHex(t, ""+
		"00000000000000000000000000000000000000000000000000000000000000ff"+ // offset = 255
		"0000000000000000000000000000000000000000000000000000000000000000"+
		"0000000000000000000000000000000000000000000000000000000000000000")
	err := unpackErr(data)
	require.Error(t, err)
	require.True(t,
		strings.Contains(err.Error(), "offset") ||
			strings.Contains(err.Error(), "length"))
}

func TestUnpackLogMessageToL2_LengthExceedsBuffer(t *testing.T) {
	// length = 5 but no element words follow.
	data := mustHex(t, ""+
		"0000000000000000000000000000000000000000000000000000000000000060"+
		"0000000000000000000000000000000000000000000000000000000000000000"+
		"0000000000000000000000000000000000000000000000000000000000000000"+
		"0000000000000000000000000000000000000000000000000000000000000005")
	require.ErrorContains(t, unpackErr(data), "exceeds buffer")
}

func TestUnpackLogMessageToL2_OffsetWordMissing(t *testing.T) {
	// Head is present but offset points past the head into nothing.
	// (No tail at all — length read should fail.)
	data := mustHex(t, ""+
		"0000000000000000000000000000000000000000000000000000000000000060"+ // offset = 96
		"0000000000000000000000000000000000000000000000000000000000000000"+
		"0000000000000000000000000000000000000000000000000000000000000000")
	require.ErrorContains(t, unpackErr(data), "length word out of range")
}

func TestUnpackLogMessageToL2_TrailingData(t *testing.T) {
	// Well-formed payload followed by one extra word of garbage.
	data := mustHex(t, ""+
		"0000000000000000000000000000000000000000000000000000000000000060"+ // offset = 96
		"0000000000000000000000000000000000000000000000000000000000000000"+ // nonce
		"0000000000000000000000000000000000000000000000000000000000000000"+ // fee
		"0000000000000000000000000000000000000000000000000000000000000000"+ // length = 0
		"deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef") // trailing
	require.ErrorContains(t, unpackErr(data), "trailing data")
}

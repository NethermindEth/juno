package utils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPadMessage_RoundTrip(t *testing.T) {
	tests := []struct {
		name          string
		msg           []byte
		numDataShards int
	}{
		{
			name:          "empty message, 1 shard",
			msg:           []byte{},
			numDataShards: 1,
		},
		{
			name:          "small message, 1 shard",
			msg:           []byte("hello"),
			numDataShards: 1,
		},
		{
			name:          "small message, 3 shards",
			msg:           []byte("hello world"),
			numDataShards: 3,
		},
		{
			name:          "message exactly divisible",
			msg:           make([]byte, 6), // varint(6)=1 byte, total=7, divisor=2*1=2 -> pad to 8
			numDataShards: 1,
		},
		{
			name:          "larger message, 10 shards",
			msg:           make([]byte, 1000),
			numDataShards: 10,
		},
		{
			name:          "single byte",
			msg:           []byte{0x42},
			numDataShards: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			padded := PadMessage(tc.msg, tc.numDataShards)

			// Verify divisibility.
			divisor := 2 * tc.numDataShards
			assert.Equal(t, 0, len(padded)%divisor,
				"padded length %d should be divisible by %d", len(padded), divisor)

			// Verify round-trip.
			recovered, err := UnpadMessage(padded)
			require.NoError(t, err)
			assert.Equal(t, tc.msg, recovered)
		})
	}
}

func TestPadMessage_Alignment(t *testing.T) {
	// Verify that padding produces the minimum size that is a multiple of divisor.
	msg := []byte("ab")          // 2 bytes
	padded := PadMessage(msg, 3) // divisor = 6
	// varint(2) = 1 byte, payload = 3 bytes, next multiple of 6 = 6
	assert.Equal(t, 6, len(padded))
}

func TestUnpadMessage_InvalidVarint(t *testing.T) {
	// An empty buffer has no valid varint.
	_, err := UnpadMessage([]byte{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid varint")
}

func TestUnpadMessage_LengthExceedsData(t *testing.T) {
	// Manually encode a varint claiming 100 bytes, but only provide 5.
	buf := make([]byte, 6)
	buf[0] = 100 // varint encoding of 100
	copy(buf[1:], "short")

	_, err := UnpadMessage(buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds available data")
}

func TestUnpadMessage_Truncated(t *testing.T) {
	// Encode a valid varint pointing past the end.
	buf := []byte{0x80, 0x01} // varint 128, but only 2 bytes total
	_, err := UnpadMessage(buf)
	assert.Error(t, err)
}

func TestPadMessage_LargeVarint(t *testing.T) {
	// A message large enough to need a multi-byte varint.
	msg := make([]byte, 300)
	for i := range msg {
		msg[i] = byte(i)
	}
	padded := PadMessage(msg, 4)
	recovered, err := UnpadMessage(padded)
	require.NoError(t, err)
	assert.Equal(t, msg, recovered)
}

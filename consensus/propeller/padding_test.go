package propeller_test

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/propeller"
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
		{
			name:          "large message requiring multi-byte varint",
			msg:           makeSequentialBytes(300),
			numDataShards: 4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			padded := propeller.PadMessage(tc.msg, tc.numDataShards)

			// Verify divisibility.
			divisor := 2 * tc.numDataShards
			assert.Equal(t, 0, len(padded)%divisor,
				"padded length %d should be divisible by %d", len(padded), divisor)

			// Verify round-trip.
			recovered, err := propeller.UnpadMessage(padded)
			require.NoError(t, err)
			assert.Equal(t, tc.msg, recovered)
		})
	}
}

func TestUnpadMessage_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr string
	}{
		{
			name:    "empty buffer",
			input:   []byte{},
			wantErr: "invalid varint",
		},
		{
			name:    "truncated varint",
			input:   []byte{0x80}, // continuation bit set, no following byte
			wantErr: "invalid varint",
		},
		{
			name:    "length exceeds data",
			input:   append([]byte{100}, []byte("short")...), // varint 100, only 5 bytes
			wantErr: "exceeds available data",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := propeller.UnpadMessage(tc.input)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestPadMessage_Size(t *testing.T) {
	msg := []byte("ab")                    // 2 bytes
	padded := propeller.PadMessage(msg, 3) // divisor = 6
	// varint(2) = 1 byte, payload = 3 bytes, next multiple of 6 = 6
	require.Len(t, padded, 6)
}

func makeSequentialBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

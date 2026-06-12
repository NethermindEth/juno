package propeller

import (
	"encoding/binary"
	"fmt"
)

// PadMessage prepends an unsigned varint-encoded length to the message and
// pads the result with zeros so the total length is divisible by
// 2*numDataShards.
//
// The varint prefix lets the receiver recover the exact original message
// length after reconstruction. The zero-padding ensures the padded message
// can be evenly split into numDataShards pieces, which is required by
// Reed-Solomon encoding (all shards must be equal length).
//
// Layout: [varint(len(msg))] [msg bytes] [zero padding]
func PadMessage(msg []byte, numDataShards int) []byte {
	// Compute the varint-encoded length prefix.
	var varintBuf [binary.MaxVarintLen64]byte
	varintLen := binary.PutUvarint(varintBuf[:], uint64(len(msg)))

	unpaddedMsgLen := uint64(varintLen + len(msg))

	// Round up to the next multiple of divisor.
	divisor := uint64(2 * numDataShards)
	paddedMsgLen := unpaddedMsgLen
	if remainder := paddedMsgLen % divisor; remainder != 0 {
		paddedMsgLen += divisor - remainder
	}

	result := make([]byte, paddedMsgLen)
	copy(result, varintBuf[:varintLen])
	copy(result[varintLen:], msg)

	return result
}

// UnpadMessage performs the reverse operation to PadMessage: it reads the varint length prefix and
// extracts the original message bytes, discarding the zero padding. The slice returned uses the
// input's backing array but re-sliced to start and finish on the original message.
//
// An error is returned if the varint is malformed or the encoded length exceeds the available data.
func UnpadMessage(padded []byte) ([]byte, error) {
	msgLen, varintLen := binary.Uvarint(padded)
	if varintLen <= 0 {
		return nil, fmt.Errorf("invalid varint prefix in padded message: %d", varintLen)
	}

	end := uint64(varintLen) + msgLen
	if end > uint64(len(padded)) {
		return nil, fmt.Errorf(
			"varint length %d exceeds available data (have %d bytes after prefix)",
			msgLen, len(padded)-varintLen,
		)
	}

	return padded[varintLen:end], nil
}

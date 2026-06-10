// Package abi decodes the non-indexed data section of specific Ethereum
// event logs the juno node consumes.
package abi

import (
	"encoding/binary"
	"fmt"
)

// WordSize is the size of a single ABI-encoded word: a uint256.
const WordSize = 32

// Word is one ABI-encoded uint256: 32 big-endian bytes. The decoder
// returns words verbatim — interpretation (numeric, address, hash) is
// the caller's job.
type Word [WordSize]byte

// UnpackLogMessageToL2 decodes the data section of the Starknet core
// contract's LogMessageToL2 event into its three non-indexed fields.
//
// Event ABI:
//
//	LogMessageToL2(address indexed fromAddress,
//	               uint256 indexed toAddress,
//	               uint256 indexed selector,
//	               uint256[] payload,
//	               uint256 nonce,
//	               uint256 fee)
//
// The indexed fields are carried in log topics, not the data buffer, and
// are extracted by the caller. The data section encodes the remaining
// (uint256[], uint256, uint256) tuple per the Solidity ABI:
//
//	head[0] (32 bytes): offset to payload tail
//	head[1] (32 bytes): nonce
//	head[2] (32 bytes): fee
//	tail at head[0]:    length (32 bytes), then length × 32 bytes of elements
func UnpackLogMessageToL2(data []byte) (payload []Word, nonce, fee Word, err error) {
	const headWords = 3
	if len(data) < headWords*WordSize {
		return nil, Word{}, Word{}, fmt.Errorf(
			"data too short for LogMessageToL2 head: %d bytes", len(data),
		)
	}

	offset, err := readOffset(data, 0)
	if err != nil {
		return nil, Word{}, Word{}, fmt.Errorf("payload offset: %w", err)
	}
	copy(nonce[:], data[WordSize:2*WordSize])
	copy(fee[:], data[2*WordSize:3*WordSize])

	payload, err = readUint256Array(data, offset)
	if err != nil {
		return nil, Word{}, Word{}, fmt.Errorf("payload: %w", err)
	}
	return payload, nonce, fee, nil
}

// readOffset reads a uint256 at data[pos:pos+32] and returns it as an int.
// Offsets that don't fit in the buffer (interpreted as a 64-bit unsigned
// integer) are rejected — a well-formed payload from a real contract has
// an offset well under 2^32.
func readOffset(data []byte, pos int) (int, error) {
	if pos+WordSize > len(data) {
		return 0, fmt.Errorf("offset word out of range")
	}
	// Top 24 bytes must be zero — anything larger overflows int.
	for _, b := range data[pos : pos+WordSize-8] {
		if b != 0 {
			return 0, fmt.Errorf("offset overflows int64")
		}
	}
	off := binary.BigEndian.Uint64(data[pos+WordSize-8 : pos+WordSize])
	if off > uint64(len(data)) {
		return 0, fmt.Errorf("offset %d out of range for buffer of %d bytes", off, len(data))
	}
	return int(off), nil //nolint:gosec // off bounded by len(data) above
}

// readUint256Array reads a dynamic uint256[] starting at data[off]:
// a length word followed by length × 32 bytes of elements.
func readUint256Array(data []byte, off int) ([]Word, error) {
	if off+WordSize > len(data) {
		return nil, fmt.Errorf("length word out of range at offset %d", off)
	}
	// Top 24 bytes of the length word must be zero.
	for _, b := range data[off : off+WordSize-8] {
		if b != 0 {
			return nil, fmt.Errorf("length overflows int64")
		}
	}
	length := binary.BigEndian.Uint64(data[off+WordSize-8 : off+WordSize])
	start := off + WordSize
	// Guard against length×32 overflowing or exceeding the buffer.
	if length > uint64(len(data)-start)/WordSize {
		return nil, fmt.Errorf(
			"array length %d exceeds buffer (need %d bytes, have %d)",
			length, length*WordSize, len(data)-start,
		)
	}
	out := make([]Word, length)
	for i := range out {
		copy(out[i][:], data[start+i*WordSize:start+(i+1)*WordSize])
	}
	return out, nil
}

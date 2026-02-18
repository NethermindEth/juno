package trieutils

import (
	"bytes"
	"encoding/binary"
)

// BitArrayOld is the same as BitArray, but implementing the methods removed
// by this PR. It'll be used to test and benchmark the new implementation.
type BitArrayOld BitArray

////////////////////////////////////////////////////////////
// Methods changed by this PR, to be tested and benchmarked
////////////////////////////////////////////////////////////

func (b *BitArrayOld) Write(buf *bytes.Buffer) (int, error) {
	n, err := buf.Write(b.ActiveBytes())

	if err := buf.WriteByte(b.len); err != nil {
		return 0, err
	}

	return n + 1, err
}

func (b *BitArrayOld) EncodedBytes() []byte {
	encode := b.ActiveBytes()
	encode = append(encode, b.len)
	return encode
}

func (b *BitArrayOld) EncodedString() string {
	bt := b.ActiveBytes()
	res := make([]byte, len(bt)+1)
	copy(res[:len(res)-1], bt)
	res[len(res)-1] = b.len
	return string(res)
}

// *** Methods copied from BitArray, used by the above methods

// same as BitArray.Bytes()
func (b *BitArrayOld) Bytes() [32]byte {
	var res [32]byte

	binary.BigEndian.PutUint64(res[0:8], b.words[3])
	binary.BigEndian.PutUint64(res[8:16], b.words[2])
	binary.BigEndian.PutUint64(res[16:24], b.words[1])
	binary.BigEndian.PutUint64(res[24:32], b.words[0])

	return res
}

// same as the new BitArray.activeBytes()
func (b *BitArrayOld) byteCount() uint {
	const bits8 = 8
	return (uint(b.len) + (bits8 - 1)) / uint(bits8)
}

// removed by this PR
func (b *BitArrayOld) ActiveBytes() []byte {
	if b.len == 0 {
		return nil
	}

	wordsBytes := b.Bytes()
	return wordsBytes[32-b.byteCount():]
}

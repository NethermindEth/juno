package collections

import (
	"bytes"
)

var EmptyBitSet = NewBitSet(0, []byte{})

type BitSet struct {
	length int
	bytes  []byte
}

func NewBitSet(length int, b []byte) *BitSet {
	bs := &BitSet{length, make([]byte, (length+7)/8)}
	offset := len(b) - len(bs.bytes)
	if offset < 0 {
		copy(bs.bytes[-offset:], b)
	} else {
		copy(bs.bytes, b[offset:])
	}
	return bs
}

func (bs *BitSet) Set(i int) {
	i += len(bs.bytes)*8 - bs.length
	bs.bytes[i/8] |= 1 << (7 - i%8)
}

func (bs *BitSet) Clear(i int) {
	i += len(bs.bytes)*8 - bs.length
	bs.bytes[i/8] &^= (1 << (7 - i%8))
}

func (bs *BitSet) Get(i int) bool {
	i += len(bs.bytes)*8 - bs.length
	return bs.bytes[i/8]&(1<<(7-i%8)) != 0
}

func (bs *BitSet) Len() int {
	return bs.length
}

func (bs *BitSet) Bytes() []byte {
	bytes := make([]byte, len(bs.bytes))
	copy(bytes, bs.bytes)
	for i := 0; i < len(bytes)*8-bs.length; i++ {
		bytes[i/8] &^= (1 << (7 - i%8))
	}
	return bytes
}

func (bs *BitSet) Equals(other *BitSet) bool {
	return bytes.Equal(bs.bytes, other.bytes)
}

func (bs *BitSet) Slice(start, end int) *BitSet {
	result := NewBitSet(end-start, []byte{})
	for i := start; i < end; i++ {
		if bs.Get(i) {
			result.Set(i - start)
		}
	}
	return result
}

func (path *BitSet) String() string {
	res := ""
	for i := 0; i < path.Len(); i++ {
		if path.Get(i) {
			res += "1"
		} else {
			res += "0"
		}
	}
	return res
}

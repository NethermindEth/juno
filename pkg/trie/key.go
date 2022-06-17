package trie

import (
	"bytes"
	"fmt"
	"math/big"
)

type Key struct {
	length int
	bytes  []byte
}

func BigToKey(length int, b *big.Int) (*Key, error) {
	// if b.BitLen() > length {
	// 	return nil, fmt.Errorf("key too large: %d > %d", b.BitLen(), length)
	// }
	// bs := &Key{length, make([]byte, (length+7)/8)}
	// b.FillBytes(bs.bytes)
	// return bs, nil
	return BytesToKey(length, b.Bytes())
}

func BytesToKey(length int, b []byte) (*Key, error) {
	b = bytes.TrimLeft(b, "\x00")
	if len(b)*8 > length {
		return nil, fmt.Errorf("key too large: %d > %d", len(b)*8, length)
	}
	bs := &Key{length, make([]byte, (length+7)/8)}
	copy(bs.bytes[len(bs.bytes)-len(b):], b)
	return bs, nil
}

func (bs *Key) Set(i int) {
	i += len(bs.bytes)*8 - bs.length
	bs.bytes[i/8] |= 1 << (7 - i%8)
}

func (bs *Key) Clear(i int) {
	i += len(bs.bytes)*8 - bs.length
	bs.bytes[i/8] &= ^(1 << (7 - i%8))
}

func (bs *Key) Get(i int) bool {
	i += len(bs.bytes)*8 - bs.length
	return bs.bytes[i/8]&(1<<(7-i%8)) != 0
}

func (bs *Key) Len() int {
	return bs.length
}

func (bs *Key) Bytes() []byte {
	return bs.bytes
}

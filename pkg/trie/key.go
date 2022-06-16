package trie

import (
	"fmt"
	"math/big"
)

type Key interface {
	Len() int
	Get(i int) bool
	Set(i int)
	Clear(i int)
}

func BigToKey(length int, b *big.Int) (Key, error) {
	if b.BitLen() > length {
		return nil, fmt.Errorf("key too large: %d > %d", b.BitLen(), length)
	}

	// implementation using bitSetKey
	bs := &bitSetKey{}
	bs.length = length
	bs.bytes = make([]byte, (length+7)/8)
	b.FillBytes(bs.bytes)
	return bs, nil

	// implementation using boolSliceKey
	// bs := &boolSliceKey{bits: make([]bool, length)}
	// for i := 0; i < b.BitLen(); i++ {
	// 	bs.bits[(length-b.BitLen())+i] = b.Bit(i) == 1
	// }
	// return bs, nil
}

// using a bool array underneath
// NOTE: please don't use this for anything other than testing
type boolSliceKey struct {
	bits []bool
}

func (k *boolSliceKey) Len() int {
	return len(k.bits)
}

func (k *boolSliceKey) Get(i int) bool {
	return k.bits[i]
}

func (k *boolSliceKey) Set(i int) {
	k.bits[i] = true
}

func (k *boolSliceKey) Clear(i int) {
	k.bits[i] = false
}

// using a byte array underneath
// TODO: probably doesn't work
type bitSetKey struct {
	length int
	bytes  []byte
}

func (bs *bitSetKey) Set(i int) {
	i += len(bs.bytes)*8 - bs.length
	bs.bytes[i/8] |= 1 << (7 - i%8)
}

func (bs *bitSetKey) Clear(i int) {
	i += len(bs.bytes)*8 - bs.length
	bs.bytes[i/8] &= ^(1 << (7 - i%8))
}

func (bs *bitSetKey) Get(i int) bool {
	i += len(bs.bytes)*8 - bs.length
	return bs.bytes[i/8]&(1<<(7-i%8)) != 0
}

func (bs *bitSetKey) Len() int {
	return bs.length
}

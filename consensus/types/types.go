package types

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
)

type Addr interface {
	~[4]uint64
}

func AddrCmp[A Addr](a, b A) bool {
	af := addrToFelt(a)
	bf := addrToFelt(b)
	return af.Cmp(&bf) == 0
}

func addrToFelt[A Addr](a A) felt.Felt {
	var b [32]byte
	for i := 0; i < 4; i++ {
		binary.BigEndian.PutUint64(b[i*8:], a[i])
	}
	return *new(felt.Felt).SetBytes(b[:])
}

type Hash interface {
	~[4]uint64
}

// Hashable's Hash() is used as ID()
type Hashable[H Hash] interface {
	Hash() H
}

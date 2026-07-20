package blake2s

import (
	"hash"
	"slices"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"golang.org/x/crypto/blake2s"
)

// Following the same implementation behind
// https://github.com/starknet-io/types-rs/blob/main/crates/starknet-types-core/src/hash/blake2s.rs

func Blake2sArray(felts []felt.Felt) felt.Hash {
	const expectedCapMult = 5

	buf := make([]byte, 0, len(felts)*expectedCapMult*4)
	for i := range felts {
		buf = appendFeltBytes(buf, &felts[i])
	}

	// Sum256 returns a fixed [32]byte value, avoiding the digest struct and result
	// slice that New256/Write/Sum would heap-allocate.
	result := blake2s.Sum256(buf)

	// Sum256 is big endian, reverse to little endian.
	slices.Reverse(result[:])
	return felt.FromBytes[felt.Hash](result[:])
}

var _ crypto.Digest = (*Blake2sDigest)(nil)

type Blake2sDigest struct {
	hasher hash.Hash
}

func NewDigest() Blake2sDigest {
	hasher, err := blake2s.New256(nil)
	if err != nil {
		panic(err)
	}
	return Blake2sDigest{hasher: hasher}
}

func (d *Blake2sDigest) Update(elems ...*felt.Felt) crypto.Digest {
	encoding := encodeFeltsToBytes(elems...)
	_, err := d.hasher.Write(encoding)
	if err != nil {
		panic(err)
	}
	return d
}

func (d *Blake2sDigest) Finish() felt.Felt {
	result := make([]byte, 0, 32)
	result = d.hasher.Sum(result)
	// Result is big endian, reverse to little endian.
	slices.Reverse(result)
	return felt.FromBytes[felt.Felt](result)
}

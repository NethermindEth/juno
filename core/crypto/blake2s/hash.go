package blake2s

import (
	"hash"
	"slices"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	"golang.org/x/crypto/blake2s"
)

// Following the same implementation behind
// https://github.com/starknet-io/types-rs/blob/main/crates/starknet-types-core/src/hash/blake2s.rs

// Blake2sArray hashes felts of any felt type and returns the hash as felt type R.
func Blake2sArray[R, F felt.FeltLike](felts []F) R {
	return R(blake2sArray(felt.CastSlice[fp.Element](felts)))
}

func blake2sArray(felts []fp.Element) fp.Element {
	buf := make([]byte, len(felts)*maxWordsPerFelt*4)
	offset := 0
	for i := range felts {
		offset += encodeFeltInto(buf[offset:], &felts[i])
	}

	// Sum256 returns a fixed [32]byte value, avoiding the digest struct and result
	// slice that New256/Write/Sum would heap-allocate.
	result := blake2s.Sum256(buf[:offset])

	// Sum256 is big endian, reverse to little endian.
	slices.Reverse(result[:])
	var hash fp.Element
	hash.SetBytes(result[:])
	return hash
}

// Blake2sDigest hashes felts incrementally. The zero value is ready to use.
type Blake2sDigest struct {
	hasher hash.Hash
}

// Update adds one element of any felt type to the digest.
func (d *Blake2sDigest) Update[F felt.FeltLike](elem *F) *Blake2sDigest {
	d.update(felt.ImplOf(elem))
	return d
}

// UpdateArray adds every element of elems to the digest.
func (d *Blake2sDigest) UpdateArray[F felt.FeltLike](elems []F) *Blake2sDigest {
	d.updateArray(felt.CastSlice[fp.Element](elems))
	return d
}

func (d *Blake2sDigest) updateArray(elems []fp.Element) {
	for i := range elems {
		d.update(&elems[i])
	}
}

func (d *Blake2sDigest) update(elem *fp.Element) {
	encoding := encodeFelts(elem)
	if _, err := d.state().Write(encoding); err != nil {
		panic(err)
	}
}

// Finish completes the digest and returns it as felt type R.
func (d *Blake2sDigest) Finish[R felt.FeltLike]() R {
	return R(d.finish())
}

func (d *Blake2sDigest) finish() fp.Element {
	result := make([]byte, 0, 32)
	result = d.state().Sum(result)
	// Result is big endian, reverse to little endian.
	slices.Reverse(result)
	var hash fp.Element
	hash.SetBytes(result)
	return hash
}

func (d *Blake2sDigest) state() hash.Hash {
	if d.hasher == nil {
		hasher, err := blake2s.New256(nil)
		if err != nil {
			panic(err)
		}
		d.hasher = hasher
	}
	return d.hasher
}

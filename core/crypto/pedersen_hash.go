package crypto

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	pedersenhash "github.com/consensys/gnark-crypto/ecc/stark-curve/pedersen-hash"
)

// PedersenArray implements [Pedersen array hashing].
//
// [Pedersen array hashing]: https://docs.starknet.io/architecture-and-concepts/cryptography/hash-functions/#array_hashing
func PedersenArray(elems ...*felt.Felt) felt.Felt {
	var digest PedersenDigest
	return digest.Update(elems...).Finish()
}

// Pedersen implements the [Pedersen hash].
//
// [Pedersen hash]: https://docs.starknet.io/architecture-and-concepts/cryptography/hash-functions/#pedersen_hash
func Pedersen(a, b *felt.Felt) felt.Felt {
	hash := pedersenhash.Pedersen(a.Impl(), b.Impl())
	return felt.Felt(hash)
}

var _ Digest = (*PedersenDigest)(nil)

type PedersenDigest struct {
	digest fp.Element
	count  uint64
}

func (d *PedersenDigest) Update(elems ...*felt.Felt) Digest {
	for idx := range elems {
		d.digest = pedersenhash.Pedersen(&d.digest, elems[idx].Impl())
	}
	d.count += uint64(len(elems))
	return d
}

func (d *PedersenDigest) Finish() felt.Felt {
	d.digest = pedersenhash.Pedersen(&d.digest, new(fp.Element).SetUint64(d.count))
	return felt.Felt(d.digest)
}

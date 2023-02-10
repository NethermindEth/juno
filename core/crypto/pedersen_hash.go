package crypto

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	pedersenhash "github.com/consensys/gnark-crypto/ecc/stark-curve/pedersen-hash"
)

// PedersenArray implements [Pedersen array hashing].
//
// [Pedersen array hashing]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#array_hashing
func PedersenArray(elems ...*felt.Felt) *felt.Felt {
	fpElements := make([]*fp.Element, len(elems))
	for i, elem := range elems {
		fpElements[i] = elem.Impl()
	}
	hash := pedersenhash.PedersenArray(fpElements...)
	return felt.NewFelt(hash)
}

// Pedersen implements the [Pedersen hash] based on the [reference implementation].
//
// [Pedersen hash]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#pedersen_hash
// [reference implementation]: https://github.com/starkware-libs/cairo-lang/blob/de741b92657f245a50caab99cfaef093152fd8be/src/starkware/crypto/signature/fast_pedersen_hash.py
func Pedersen(a *felt.Felt, b *felt.Felt) *felt.Felt {
	hash := pedersenhash.Pedersen(a.Impl(), b.Impl())
	return felt.NewFelt(hash)
}

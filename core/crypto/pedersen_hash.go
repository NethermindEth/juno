package crypto

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	pedersenhash "github.com/consensys/gnark-crypto/ecc/stark-curve/pedersen-hash"
	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PedersenArray implements [Pedersen array hashing].
//
// [Pedersen array hashing]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#array_hashing
func PedersenArray(elems ...*felt.Felt) *felt.Felt {
	var digest PedersenDigest
	return digest.Update(elems...).Finish()
}

var lruPederson, _ = lru.New(100000000)

var pedersonCache = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "juno_pederson",
	Help: "pederson",
}, []string{"hit"})

// Pedersen implements the [Pedersen hash].
//
// [Pedersen hash]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#pedersen_hash
func Pedersen(a, b *felt.Felt) *felt.Felt {
	key := lruKey{
		x: *a, y: *b,
	}

	res, ok := lruPederson.Get(key)
	if ok {
		pedersonCache.WithLabelValues("true").Inc()
		return res.(*felt.Felt)
	}

	hash := pedersenhash.Pedersen(a.Impl(), b.Impl())
	result := felt.NewFelt(&hash)
	lruPederson.Add(key, result)
	pedersonCache.WithLabelValues("false").Inc()
	return result
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

func (d *PedersenDigest) Finish() *felt.Felt {
	d.digest = pedersenhash.Pedersen(&d.digest, new(fp.Element).SetUint64(d.count))
	return felt.NewFelt(&d.digest)
}

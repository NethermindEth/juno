package crypto

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
	pedersenhash "github.com/consensys/gnark-crypto/ecc/stark-curve/pedersen-hash"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"time"
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
	return felt.NewFelt(&hash)
}

var pedersonTime = promauto.NewCounter(prometheus.CounterOpts{
	Name: "juno_pederson_time",
	Help: "Time in address get",
})
var pedersonCount = promauto.NewCounter(prometheus.CounterOpts{
	Name: "juno_pederson_count",
	Help: "Time in address get",
})

// Pedersen implements the [Pedersen hash].
//
// [Pedersen hash]: https://docs.starknet.io/documentation/develop/Hashing/hash-functions/#pedersen_hash
func Pedersen(a, b *felt.Felt) *felt.Felt {
	pedersonCount.Inc()
	starttime := time.Now()
	defer func() {
		pedersonTime.Add(float64(time.Now().Sub(starttime).Microseconds()))
	}()

	hash := pedersenhash.Pedersen(a.Impl(), b.Impl())
	return felt.NewFelt(&hash)
}

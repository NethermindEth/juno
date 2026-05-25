package proposal_test

import (
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
)

const (
	benchHeights = 5
	benchN       = 1000
)

func heightFor(i int) uint64 {
	return uint64(i%benchHeights) + 1
}

func BenchmarkStoreThenGet(b *testing.B) {
	b.ReportAllocs()
	s := &proposal.ProposalStore[starknet.Hash]{}
	val := buildResultAtHeight(1)
	keys := make([]starknet.Hash, b.N)
	for i := range keys {
		keys[i] = felt.FromUint64[starknet.Hash](uint64(i))
	}
	b.ResetTimer()
	for i := range b.N {
		s.Store(keys[i], val)
		_ = s.Get(keys[i])
	}
}

func BenchmarkParallelGet(b *testing.B) {
	b.ReportAllocs()
	s := &proposal.ProposalStore[starknet.Hash]{}
	for i := range benchN {
		s.Store(felt.FromUint64[starknet.Hash](uint64(i)), buildResultAtHeight(heightFor(i)))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := felt.FromUint64[starknet.Hash](rand.Uint64N(benchN))
			_ = s.Get(key)
		}
	})
}

func BenchmarkParallelMixed(b *testing.B) {
	b.ReportAllocs()
	s := &proposal.ProposalStore[starknet.Hash]{}
	for i := range benchN {
		s.Store(felt.FromUint64[starknet.Hash](uint64(i)), buildResultAtHeight(heightFor(i)))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			k := rand.Uint64N(benchN * 2)
			key := felt.FromUint64[starknet.Hash](k)
			if k%10 == 0 {
				s.Store(key, buildResultAtHeight(heightFor(int(k))))
			} else {
				_ = s.Get(key)
			}
		}
	})
}

func BenchmarkFinalizeHeight(b *testing.B) {
	b.ReportAllocs()
	const (
		liveHeights     = 16
		hashesPerHeight = 64
	)
	for range b.N {
		b.StopTimer()
		s := &proposal.ProposalStore[starknet.Hash]{}
		for h := range liveHeights {
			for k := range hashesPerHeight {
				id := uint64(h)*hashesPerHeight + uint64(k)
				s.Store(felt.FromUint64[starknet.Hash](id), buildResultAtHeight(uint64(h)+1))
			}
		}
		b.StartTimer()
		s.FinalizeHeight(types.Height(liveHeights))
	}
}

package proposal_test

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
)

type legacyStore[H types.Hash] struct {
	underlying sync.Map
}

func (p *legacyStore[H]) Get(key H) *builder.BuildResult {
	value, ok := p.underlying.Load(key)
	if !ok {
		return nil
	}
	return value.(*builder.BuildResult)
}

func (p *legacyStore[H]) Store(key H, value *builder.BuildResult) {
	if value == nil {
		return
	}
	_, _ = p.underlying.LoadOrStore(key, value)
}

func (p *legacyStore[H]) DeleteUpToHeight(height types.Height) {
	p.underlying.Range(func(key, value any) bool {
		br, ok := value.(*builder.BuildResult)
		if !ok {
			return true
		}
		if types.Height(br.PreConfirmed.Block.Number) <= height {
			p.underlying.Delete(key)
		}
		return true
	})
}

func BenchmarkNewStore_StoreThenGet(b *testing.B) {
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

func BenchmarkLegacy_StoreThenGet(b *testing.B) {
	s := &legacyStore[starknet.Hash]{}
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

const (
	benchHeights = 5
	benchN       = 1000
)

func heightFor(i int) uint64 {
	return uint64(i%benchHeights) + 1
}

func BenchmarkNewStore_ParallelGet(b *testing.B) {
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

func BenchmarkLegacy_ParallelGet(b *testing.B) {
	s := &legacyStore[starknet.Hash]{}
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

func BenchmarkNewStore_ParallelMixed(b *testing.B) {
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

func BenchmarkLegacy_ParallelMixed(b *testing.B) {
	s := &legacyStore[starknet.Hash]{}
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

func BenchmarkNewStore_DeleteUpToHeight(b *testing.B) {
	const (
		liveHeights     = 16
		hashesPerHeight = 64
	)
	for i := range b.N {
		_ = i
		b.StopTimer()
		s := &proposal.ProposalStore[starknet.Hash]{}
		for h := range liveHeights {
			for k := range hashesPerHeight {
				id := uint64(h)*hashesPerHeight + uint64(k)
				s.Store(felt.FromUint64[starknet.Hash](id), buildResultAtHeight(uint64(h)+1))
			}
		}
		b.StartTimer()
		s.DeleteUpToHeight(types.Height(liveHeights))
	}
}

func BenchmarkLegacy_DeleteUpToHeight(b *testing.B) {
	const (
		liveHeights     = 16
		hashesPerHeight = 64
	)
	for i := range b.N {
		_ = i
		b.StopTimer()
		s := &legacyStore[starknet.Hash]{}
		for h := range liveHeights {
			for k := range hashesPerHeight {
				id := uint64(h)*hashesPerHeight + uint64(k)
				s.Store(felt.FromUint64[starknet.Hash](id), buildResultAtHeight(uint64(h)+1))
			}
		}
		b.StartTimer()
		s.DeleteUpToHeight(types.Height(liveHeights))
	}
}

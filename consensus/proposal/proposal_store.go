package proposal

import (
	"sync"
	"sync/atomic"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
)

type ProposalStore[H types.Hash] struct {
	byHeight  sync.Map // map[types.Height]*sync.Map[H]*builder.BuildResult
	finalized atomic.Uint64
}

func (p *ProposalStore[H]) Get(key H) *builder.BuildResult {
	var found *builder.BuildResult
	p.byHeight.Range(func(h, bucket any) bool {
		v, ok := bucket.(*sync.Map).Load(key)
		if !ok {
			return true
		}
		if uint64(h.(types.Height)) <= p.finalized.Load() {
			return false
		}
		found = v.(*builder.BuildResult)
		return false
	})
	return found
}

func (p *ProposalStore[H]) Store(key H, value *builder.BuildResult) {
	if value == nil {
		return
	}
	height := types.Height(value.PreConfirmed.Block.Number)
	if uint64(height) <= p.finalized.Load() {
		return
	}
	bucket := p.bucketFor(height)
	bucket.LoadOrStore(key, value)

	if uint64(height) <= p.finalized.Load() {
		bucket.CompareAndDelete(key, value)
		p.byHeight.CompareAndDelete(height, bucket)
	}
}

func (p *ProposalStore[H]) bucketFor(height types.Height) *sync.Map {
	if existing, ok := p.byHeight.Load(height); ok {
		return existing.(*sync.Map)
	}
	actual, _ := p.byHeight.LoadOrStore(height, &sync.Map{})
	return actual.(*sync.Map)
}

func (p *ProposalStore[H]) IsFinalized(height types.Height) bool {
	return uint64(height) <= p.finalized.Load()
}

func (p *ProposalStore[H]) DeleteUpToHeight(height types.Height) {
	for {
		cur := p.finalized.Load()
		if uint64(height) <= cur {
			return
		}
		if p.finalized.CompareAndSwap(cur, uint64(height)) {
			break
		}
	}
	p.byHeight.Range(func(h, _ any) bool {
		if h.(types.Height) <= height {
			p.byHeight.Delete(h)
		}
		return true
	})
}

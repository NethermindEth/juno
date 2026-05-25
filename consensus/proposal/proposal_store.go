package proposal

import (
	"sync"
	"sync/atomic"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
)

type ProposalStore[H types.Hash] struct {
	proposalsByHeight sync.Map // height -> hash -> BuildResult
	finalizedHeight   atomic.Uint64
}

func (p *ProposalStore[H]) Get(key H) *builder.BuildResult {
	var found *builder.BuildResult
	p.proposalsByHeight.Range(func(height, proposals any) bool {
		buildResult, ok := proposals.(*sync.Map).Load(key)
		if !ok {
			return true
		}
		if p.IsFinalized(height.(types.Height)) {
			return false
		}
		found = buildResult.(*builder.BuildResult)
		return false
	})
	return found
}

func (p *ProposalStore[H]) Store(key H, builtResult *builder.BuildResult) {
	if builtResult == nil {
		return
	}
	height := types.Height(builtResult.PreConfirmed.Block.Number)
	if p.IsFinalized(height) {
		return
	}
	bucket := p.proposalFor(height)
	bucket.LoadOrStore(key, builtResult)

	if p.IsFinalized(height) {
		bucket.CompareAndDelete(key, builtResult)
		p.proposalsByHeight.CompareAndDelete(height, bucket)
	}
}

func (p *ProposalStore[H]) proposalFor(height types.Height) *sync.Map {
	if existing, ok := p.proposalsByHeight.Load(height); ok {
		return existing.(*sync.Map)
	}
	actual, _ := p.proposalsByHeight.LoadOrStore(height, &sync.Map{})
	return actual.(*sync.Map)
}

func (p *ProposalStore[H]) IsFinalized(height types.Height) bool {
	return uint64(height) <= p.finalizedHeight.Load()
}

func (p *ProposalStore[H]) FinalizeHeight(height types.Height) {
	for {
		cur := p.finalizedHeight.Load()
		if uint64(height) <= cur {
			return
		}
		if p.finalizedHeight.CompareAndSwap(cur, uint64(height)) {
			break
		}
	}
	p.proposalsByHeight.Range(func(h, _ any) bool {
		if h.(types.Height) <= height {
			p.proposalsByHeight.Delete(h)
		}
		return true
	})
}

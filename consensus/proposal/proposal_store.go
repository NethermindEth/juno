package proposal

import (
	syncmap "sync"

	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
)

type ProposalStore[H types.Hash] struct {
	underlying syncmap.Map
}

func (p *ProposalStore[H]) Get(key H) *builder.BuildResult {
	value, ok := p.underlying.Load(key)
	if !ok {
		return nil
	}

	buildResult, ok := value.(*builder.BuildResult)
	if !ok {
		return nil
	}

	return buildResult
}

func (p *ProposalStore[H]) Store(key H, value *builder.BuildResult) {
	if value == nil {
		return
	}
	_, _ = p.underlying.LoadOrStore(key, value)
}

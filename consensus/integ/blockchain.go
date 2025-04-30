package integ

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
)

type blockchain struct {
	height   types.Height
	nodeAddr felt.Felt
	commits  chan commit
}

type commit struct {
	nodeAddr felt.Felt
	height   types.Height
	value    value
}

func newBlockchain(commits chan commit, nodeAddr felt.Felt) *blockchain {
	return &blockchain{
		height:   0,
		nodeAddr: nodeAddr,
		commits:  commits,
	}
}

func (b *blockchain) Height() types.Height {
	return b.height
}

func (b *blockchain) Commit(height types.Height, value value, precommits []types.Precommit[felt.Felt, felt.Felt]) {
	b.height = max(b.height, height)
	b.commits <- commit{
		nodeAddr: b.nodeAddr,
		height:   height,
		value:    value,
	}
}

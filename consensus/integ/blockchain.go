package integ

import (
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/core/felt"
)

type blockchain struct {
	height   tendermint.Height
	nodeAddr felt.Felt
	commits  chan commit
}

type commit struct {
	nodeAddr felt.Felt
	height   tendermint.Height
	value    value
}

func newBlockchain(commits chan commit, nodeAddr felt.Felt) *blockchain {
	return &blockchain{
		height:   0,
		nodeAddr: nodeAddr,
		commits:  commits,
	}
}

func (b *blockchain) Height() tendermint.Height {
	return b.height
}

func (b *blockchain) Commit(height tendermint.Height, value value, precommits []tendermint.Precommit[felt.Felt, felt.Felt]) {
	b.height = max(b.height, height)
	b.commits <- commit{
		nodeAddr: b.nodeAddr,
		height:   height,
		value:    value,
	}
}

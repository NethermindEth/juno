package integtest

import (
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
)

type blockchain struct {
	height   types.Height
	nodeAddr *types.Addr
	commits  chan commit
}

type commit struct {
	nodeAddr *types.Addr
	height   types.Height
	value    starknet.Value
}

func newBlockchain(commits chan commit, nodeAddr *types.Addr) *blockchain {
	return &blockchain{
		height:   0,
		nodeAddr: nodeAddr,
		commits:  commits,
	}
}

func (b *blockchain) Height() types.Height {
	return b.height
}

func (b *blockchain) Commit(height types.Height, value starknet.Value) {
	b.height = max(b.height, height)
	b.commits <- commit{
		nodeAddr: b.nodeAddr,
		height:   height,
		value:    value,
	}
}

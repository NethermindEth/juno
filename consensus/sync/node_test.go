package sync_test

import (
	"math/rand/v2"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
)

type nodes []int

func newNodes(nodeCount int) nodes {
	return nodes(rand.Perm(nodeCount))
}

func (n nodes) TotalVotingPower(height types.Height) types.VotingPower {
	return types.VotingPower(len(n))
}

func (n nodes) ValidatorVotingPower(height types.Height, addr *starknet.Address) types.VotingPower {
	return types.VotingPower(1)
}

// Randomised proposer selection, with prime coefficients so that for each height, the order of proposers is different.
func (n nodes) Proposer(height types.Height, round types.Round) starknet.Address {
	nodeIndex := (int(height)*17 + int(round)*31) % len(n)
	nodeID := n[nodeIndex]
	return felt.FromUint64[starknet.Address](uint64(nodeID))
}

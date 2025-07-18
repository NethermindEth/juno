package integtest

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

func (n nodes) ValidatorVotingPower(addr starknet.Address) types.VotingPower {
	return types.VotingPower(1)
}

// Randomised proposer selection, with prime coefficients so that for each height, the order of proposers is different.
func (n nodes) Proposer(height types.Height, round types.Round) starknet.Address {
	idx := (int(height)*31 + int(round)*17) % len(n)
	return testAddress(n[idx])
}

func testAddress(i int) starknet.Address {
	return starknet.Address(felt.FromUint64(uint64(i)))
}

func testAddressIndex(addr *starknet.Address) int {
	return int(addr.AsFelt().Uint64())
}

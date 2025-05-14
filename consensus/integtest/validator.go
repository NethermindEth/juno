package integtest

import (
	"math/rand"
	"slices"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
)

type validators []felt.Felt

func newValidators(allNodes nodes) validators {
	addresses := slices.Clone(allNodes.addr)
	rand.Shuffle(len(addresses), func(i, j int) {
		addresses[i], addresses[j] = addresses[j], addresses[i]
	})
	return validators(addresses)
}

func (v validators) TotalVotingPower(height types.Height) types.VotingPower {
	return types.VotingPower(len(v))
}

func (v validators) ValidatorVotingPower(addr felt.Felt) types.VotingPower {
	return types.VotingPower(1)
}

// Randomised proposer selection, with prime coefficients so that for each height, the order of proposers is different.
func (v validators) Proposer(height types.Height, round types.Round) felt.Felt {
	idx := (int(height)*31 + int(round)*17) % len(v)
	return v[idx]
}

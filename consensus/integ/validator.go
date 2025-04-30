package integ

import (
	"math/rand"
	"slices"

	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/core/felt"
)

type validators struct {
	allocations []felt.Felt
}

func newValidators(allNodes nodes) *validators {
	allocations := slices.Clone(allNodes.addr)
	rand.Shuffle(len(allocations), func(i, j int) {
		allocations[i], allocations[j] = allocations[j], allocations[i]
	})
	return &validators{
		allocations: allocations,
	}
}

func (v *validators) TotalVotingPower(height tendermint.Height) tendermint.VotingPower {
	return tendermint.VotingPower(len(v.allocations))
}

func (v *validators) ValidatorVotingPower(addr felt.Felt) tendermint.VotingPower {
	return tendermint.VotingPower(1)
}

func (v *validators) Proposer(height tendermint.Height, round tendermint.Round) felt.Felt {
	idx := (int(height)*31 + int(round)*17) % len(v.allocations)
	return v.allocations[idx]
}

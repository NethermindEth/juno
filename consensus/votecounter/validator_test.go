package votecounter

import (
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
)

type mockValidator struct {
	proposer    *starknet.Address
	votingPower map[starknet.Address]types.VotingPower
}

func newMockValidator(proposerIndex uint64, votingPower map[starknet.Address]types.VotingPower) *mockValidator {
	return &mockValidator{
		proposer:    felt.NewFromUint64[starknet.Address](proposerIndex),
		votingPower: votingPower,
	}
}

func (v mockValidator) TotalVotingPower(height types.Height) types.VotingPower {
	total := types.VotingPower(0)
	for _, power := range v.votingPower {
		total += power
	}
	return total
}

func (v mockValidator) ValidatorVotingPower(height types.Height, addr *starknet.Address) types.VotingPower {
	return v.votingPower[*addr]
}

func (v mockValidator) Proposer(height types.Height, round types.Round) starknet.Address {
	return *v.proposer
}

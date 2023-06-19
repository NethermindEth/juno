package blockchain

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type Pending struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
	NewClasses  map[felt.Felt]core.Class
}

type PendingState struct {
	pending Pending
	head    core.StateReader
}

func NewPendingState(pending Pending, head core.StateReader) *PendingState {
	return &PendingState{
		pending: pending,
		head:    head,
	}
}

func (p *PendingState) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	for _, replaced := range p.pending.StateUpdate.StateDiff.ReplacedClasses {
		if replaced.Address.Equal(addr) {
			return replaced.ClassHash, nil
		}
	}

	for _, deployed := range p.pending.StateUpdate.StateDiff.DeployedContracts {
		if deployed.Address.Equal(addr) {
			return deployed.ClassHash, nil
		}
	}

	return p.head.ContractClassHash(addr)
}

func (p *PendingState) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	if nonce, found := p.pending.StateUpdate.StateDiff.Nonces[*addr]; found {
		return nonce, nil
	}

	return p.head.ContractNonce(addr)
}

func (p *PendingState) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	if diffs, found := p.pending.StateUpdate.StateDiff.StorageDiffs[*addr]; found {
		for _, diff := range diffs {
			if diff.Key.Equal(key) {
				return diff.Value, nil
			}
		}
	}

	return p.head.ContractStorage(addr, key)
}

func (p *PendingState) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	if class, found := p.pending.NewClasses[*classHash]; found {
		return &core.DeclaredClass{
			At:    0,
			Class: class,
		}, nil
	}

	return p.head.Class(classHash)
}

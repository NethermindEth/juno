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
	stateDiff  *core.StateDiff
	newClasses map[felt.Felt]core.Class
	head       core.StateReader
}

func NewPendingState(stateDiff *core.StateDiff, newClasses map[felt.Felt]core.Class, head core.StateReader) *PendingState {
	return &PendingState{
		stateDiff:  stateDiff,
		newClasses: newClasses,
		head:       head,
	}
}

func (p *PendingState) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	for _, replaced := range p.stateDiff.ReplacedClasses {
		if replaced.Address.Equal(addr) {
			return replaced.ClassHash, nil
		}
	}

	for _, deployed := range p.stateDiff.DeployedContracts {
		if deployed.Address.Equal(addr) {
			return deployed.ClassHash, nil
		}
	}

	return p.head.ContractClassHash(addr)
}

func (p *PendingState) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	if nonce, found := p.stateDiff.Nonces[*addr]; found {
		return nonce, nil
	}

	if p.isDeployedInPending(addr) {
		return &felt.Zero, nil
	}
	return p.head.ContractNonce(addr)
}

func (p *PendingState) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	if diffs, found := p.stateDiff.StorageDiffs[*addr]; found {
		for _, diff := range diffs {
			if diff.Key.Equal(key) {
				return diff.Value, nil
			}
		}
	}

	if p.isDeployedInPending(addr) {
		return &felt.Zero, nil
	}
	return p.head.ContractStorage(addr, key)
}

func (p *PendingState) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	if class, found := p.newClasses[*classHash]; found {
		return &core.DeclaredClass{
			At:    0,
			Class: class,
		}, nil
	}

	return p.head.Class(classHash)
}

func (p *PendingState) isDeployedInPending(addr *felt.Felt) bool {
	for _, deployed := range p.stateDiff.DeployedContracts {
		if deployed.Address.Equal(addr) {
			return true
		}
	}
	return false
}

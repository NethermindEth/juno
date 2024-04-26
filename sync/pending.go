package sync

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
	if classHash, ok := p.stateDiff.ReplacedClasses[*addr]; ok {
		return classHash, nil
	} else if classHash, ok = p.stateDiff.DeployedContracts[*addr]; ok {
		return classHash, nil
	}
	return p.head.ContractClassHash(addr)
}

func (p *PendingState) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	if nonce, found := p.stateDiff.Nonces[*addr]; found {
		return nonce, nil
	} else if _, found = p.stateDiff.DeployedContracts[*addr]; found {
		return &felt.Felt{}, nil
	}
	return p.head.ContractNonce(addr)
}

func (p *PendingState) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	if diffs, found := p.stateDiff.StorageDiffs[*addr]; found {
		if value, found := diffs[*key]; found {
			return value, nil
		}
	}
	if _, found := p.stateDiff.DeployedContracts[*addr]; found {
		return &felt.Felt{}, nil
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

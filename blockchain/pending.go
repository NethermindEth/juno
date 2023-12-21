package blockchain

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
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

func (p *PendingState) SetStorage(contractAddress, key, value *felt.Felt) error {
	if _, found := p.stateDiff.StorageDiffs[*contractAddress]; !found {
		p.stateDiff.StorageDiffs[*contractAddress] = make(map[felt.Felt]*felt.Felt)
	}
	p.stateDiff.StorageDiffs[*contractAddress][*key] = value.Clone()
	return nil
}

func (p *PendingState) IncrementNonce(contractAddress *felt.Felt) error {
	currentNonce, err := p.ContractNonce(contractAddress)
	if err != nil {
		return fmt.Errorf("get contract nonce: %v", err)
	}
	newNonce := new(felt.Felt).SetUint64(1)
	p.stateDiff.Nonces[*contractAddress] = newNonce.Add(currentNonce, newNonce)
	return nil
}

func (p *PendingState) SetClassHash(contractAddress, classHash *felt.Felt) error {
	if _, err := p.head.ContractClassHash(contractAddress); err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			p.stateDiff.DeployedContracts[*contractAddress] = classHash.Clone()
			return nil
		}
		return fmt.Errorf("get latest class hash: %v", err)
	}
	p.stateDiff.ReplacedClasses[*contractAddress] = classHash.Clone()
	return nil
}

func (p *PendingState) SetContractClass(classHash *felt.Felt, class core.Class) error {
	if _, err := p.Class(classHash); err == nil {
		return errors.New("class already declared")
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return fmt.Errorf("get class: %v", err)
	}

	p.newClasses[*classHash] = class
	if class.Version() == 0 {
		p.stateDiff.DeclaredV0Classes = append(p.stateDiff.DeclaredV0Classes, classHash.Clone())
	} // assumption: SetCompiledClassHash will be called for Cairo1 contracts
	return nil
}

func (p *PendingState) SetCompiledClassHash(classHash, compiledClassHash *felt.Felt) error {
	// assumption: SetContractClass was called for classHash and succeeded
	p.stateDiff.DeclaredV1Classes[*classHash] = compiledClassHash.Clone()
	return nil
}

// StateDiff returns the pending state's internal state diff. The returned object will continue to be
// read and modified by the pending state.
func (p *PendingState) StateDiff() *core.StateDiff {
	return p.stateDiff
}

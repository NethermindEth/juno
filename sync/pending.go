package sync

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
)

var feltOne = new(felt.Felt).SetUint64(1)

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

func (p *PendingState) ChainHeight() (uint64, error) {
	return p.head.ChainHeight()
}

func (p *PendingState) StateDiff() *core.StateDiff {
	return p.stateDiff
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

func (p *PendingState) ClassTrie() (*trie.Trie, error) {
	return nil, core.ErrHistoricalTrieNotSupported
}

func (p *PendingState) ContractTrie() (*trie.Trie, error) {
	return nil, core.ErrHistoricalTrieNotSupported
}

func (p *PendingState) ContractStorageTrie(addr *felt.Felt) (*trie.Trie, error) {
	return nil, core.ErrHistoricalTrieNotSupported
}

type PendingStateWriter struct {
	*PendingState
}

func NewPendingStateWriter(stateDiff *core.StateDiff, newClasses map[felt.Felt]core.Class, head core.StateReader) PendingStateWriter {
	return PendingStateWriter{
		PendingState: &PendingState{
			stateDiff:  stateDiff,
			newClasses: newClasses,
			head:       head,
		},
	}
}

func (p *PendingStateWriter) SetStorage(contractAddress, key, value *felt.Felt) error {
	if _, found := p.stateDiff.StorageDiffs[*contractAddress]; !found {
		p.stateDiff.StorageDiffs[*contractAddress] = make(map[felt.Felt]*felt.Felt)
	}
	p.stateDiff.StorageDiffs[*contractAddress][*key] = value.Clone()
	return nil
}

func (p *PendingStateWriter) IncrementNonce(contractAddress *felt.Felt) error {
	currentNonce, err := p.ContractNonce(contractAddress)
	if err != nil {
		return fmt.Errorf("get contract nonce: %v", err)
	}
	p.stateDiff.Nonces[*contractAddress] = currentNonce.Add(currentNonce, feltOne)
	return nil
}

func (p *PendingStateWriter) SetClassHash(contractAddress, classHash *felt.Felt) error {
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

// SetContractClass writes a new CairoV0 class to the PendingState
// Assumption: SetCompiledClassHash should be called for CairoV1 contracts
func (p *PendingStateWriter) SetContractClass(classHash *felt.Felt, class core.Class) error {
	// Only declare the class if it has not already been declared, and return
	// and unexepcted errors (ie any error that isn't db.ErrKeyNotFound)
	_, err := p.Class(classHash)
	if err == nil {
		return errors.New("class already declared")
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return fmt.Errorf("get class: %v", err)
	}

	p.newClasses[*classHash] = class
	if class.Version() == 0 {
		p.stateDiff.DeclaredV0Classes = append(p.stateDiff.DeclaredV0Classes, classHash.Clone())
	}
	return nil
}

// SetCompiledClassHash writes CairoV1 classes to the pending state
// Assumption: SetContractClass was called for classHash and succeeded
func (p *PendingStateWriter) SetCompiledClassHash(classHash, compiledClassHash *felt.Felt) error {
	p.stateDiff.DeclaredV1Classes[*classHash] = compiledClassHash.Clone()
	return nil
}

// StateDiffAndClasses returns the pending state's internal data. The returned objects will continue to be
// read and modified by the pending state.
func (p *PendingStateWriter) StateDiffAndClasses() (core.StateDiff, map[felt.Felt]core.Class) {
	return *p.stateDiff, p.newClasses
}

func (p *PendingStateWriter) SetStateDiff(stateDiff *core.StateDiff) {
	p.stateDiff = stateDiff
}

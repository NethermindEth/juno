package pending

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

var ErrHistoricalTrieNotSupported = errors.New("cannot support historical trie")

type State struct {
	stateDiff  *core.StateDiff
	newClasses map[felt.Felt]core.ClassDefinition
	head       core.StateReader

	// The block number of the pending block that this
	// state represents
	blockNumber uint64
}

func NewState(
	stateDiff *core.StateDiff,
	newClasses map[felt.Felt]core.ClassDefinition,
	head core.StateReader,
	blockNumber uint64,
) *State {
	return &State{
		stateDiff:   stateDiff,
		newClasses:  newClasses,
		head:        head,
		blockNumber: blockNumber,
	}
}

func (p *State) StateDiff() *core.StateDiff {
	return p.stateDiff
}

func (p *State) ContractClassHash(addr *felt.Felt) (felt.Felt, error) {
	if classHash, ok := p.stateDiff.ReplacedClasses[*addr]; ok {
		return *classHash, nil
	} else if classHash, ok = p.stateDiff.DeployedContracts[*addr]; ok {
		return *classHash, nil
	}
	return p.head.ContractClassHash(addr)
}

func (p *State) ContractNonce(addr *felt.Felt) (felt.Felt, error) {
	if nonce, found := p.stateDiff.Nonces[*addr]; found {
		return *nonce, nil
	} else if _, found = p.stateDiff.DeployedContracts[*addr]; found {
		return felt.Felt{}, nil
	}
	return p.head.ContractNonce(addr)
}

func (p *State) ContractStorage(addr, key *felt.Felt) (felt.Felt, error) {
	if diffs, found := p.stateDiff.StorageDiffs[*addr]; found {
		if value, found := diffs[*key]; found {
			return *value, nil
		}
	}
	if _, found := p.stateDiff.DeployedContracts[*addr]; found {
		return felt.Felt{}, nil
	}
	return p.head.ContractStorage(addr, key)
}

// ContractStorageLastUpdatedBlock returns the most recent block number at which a given storage
// slot key of a given contract was last updated.
func (p *State) ContractStorageLastUpdatedBlock(
	addr *felt.Address,
	key *felt.Felt,
) (uint64, error) {
	addrFelt := felt.Felt(*addr)
	if diffs, found := p.stateDiff.StorageDiffs[addrFelt]; found {
		if _, found := diffs[*key]; found {
			return p.blockNumber, nil
		}
	}
	if _, found := p.stateDiff.DeployedContracts[addrFelt]; found {
		return 0, nil
	}
	return p.head.ContractStorageLastUpdatedBlock(addr, key)
}

func (p *State) Class(classHash *felt.Felt) (*core.DeclaredClassDefinition, error) {
	if class, found := p.newClasses[*classHash]; found {
		return &core.DeclaredClassDefinition{
			At:    0,
			Class: class,
		}, nil
	}

	return p.head.Class(classHash)
}

func (p *State) CompiledClassHash(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	classHashFelt := felt.Felt(*classHash)
	if casmHash, found := p.stateDiff.DeclaredV1Classes[classHashFelt]; found {
		return felt.CasmClassHash(*casmHash), nil
	}
	return p.head.CompiledClassHash(classHash)
}

func (p *State) CompiledClassHashV2(
	classHash *felt.SierraClassHash,
) (felt.CasmClassHash, error) {
	if casmHash, found := p.stateDiff.MigratedClasses[*classHash]; found {
		return casmHash, nil
	}
	return p.head.CompiledClassHashV2(classHash)
}

func (p *State) ClassTrie() (core.TrieReader, error) {
	return nil, ErrHistoricalTrieNotSupported
}

func (p *State) ContractTrie() (core.TrieReader, error) {
	return nil, ErrHistoricalTrieNotSupported
}

func (p *State) ContractStorageTrie(addr *felt.Felt) (core.TrieReader, error) {
	return nil, ErrHistoricalTrieNotSupported
}

type StateWriter struct {
	*State
}

func NewStateWriter(
	stateDiff *core.StateDiff,
	newClasses map[felt.Felt]core.ClassDefinition,
	head core.StateReader,
) StateWriter {
	return StateWriter{
		State: &State{
			stateDiff:  stateDiff,
			newClasses: newClasses,
			head:       head,
		},
	}
}

func (p *StateWriter) SetStorage(contractAddress, key, value *felt.Felt) error {
	if _, found := p.stateDiff.StorageDiffs[*contractAddress]; !found {
		p.stateDiff.StorageDiffs[*contractAddress] = make(map[felt.Felt]*felt.Felt)
	}
	p.stateDiff.StorageDiffs[*contractAddress][*key] = value.Clone()
	return nil
}

func (p *StateWriter) IncrementNonce(contractAddress *felt.Felt) error {
	currentNonce, err := p.ContractNonce(contractAddress)
	if err != nil {
		return fmt.Errorf("get contract nonce: %v", err)
	}
	p.stateDiff.Nonces[*contractAddress] = currentNonce.Add(&currentNonce, &felt.One)
	return nil
}

func (p *StateWriter) SetClassHash(contractAddress, classHash *felt.Felt) error {
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
func (p *StateWriter) SetContractClass(classHash *felt.Felt, class core.ClassDefinition) error {
	// Only declare the class if it has not already been declared, and return
	// and unexepcted errors (ie any error that isn't db.ErrKeyNotFound)
	_, err := p.Class(classHash)
	if err == nil {
		return errors.New("class already declared")
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return fmt.Errorf("get class: %v", err)
	}

	p.newClasses[*classHash] = class
	if _, ok := class.(*core.DeprecatedCairoClass); ok {
		p.stateDiff.DeclaredV0Classes = append(p.stateDiff.DeclaredV0Classes, classHash.Clone())
	}
	return nil
}

// SetCompiledClassHash writes CairoV1 classes to the pending state
// Assumption: SetContractClass was called for classHash and succeeded
func (p *StateWriter) SetCompiledClassHash(classHash, compiledClassHash *felt.Felt) error {
	p.stateDiff.DeclaredV1Classes[*classHash] = compiledClassHash.Clone()
	return nil
}

// StateDiffAndClasses returns the pending state's internal data. The returned objects will continue to be
// read and modified by the pending state.
func (p *StateWriter) StateDiffAndClasses() (core.StateDiff, map[felt.Felt]core.ClassDefinition) {
	return *p.stateDiff, p.newClasses
}

func (p *StateWriter) SetStateDiff(stateDiff *core.StateDiff) {
	p.stateDiff = stateDiff
}

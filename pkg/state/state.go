package state

import (
	"encoding/json"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/types"
)

const (
	StateTrieHeight   = 251
	StorageTrieHeight = 251
)

type State interface {
	Root() *felt.Felt
	GetContractState(address *felt.Felt) (*ContractState, error)
	GetContract(address *felt.Felt) (*types.Contract, error)
	SetContract(address *felt.Felt, hash *felt.Felt, code *types.Contract) error
	GetSlot(address *felt.Felt, slot *felt.Felt) (*felt.Felt, error)
	SetSlot(address *felt.Felt, slot *felt.Felt, value *felt.Felt) error
	GetClassHash(address *felt.Felt) (*felt.Felt, error)
	GetClass(blockId any, classHash *felt.Felt) (*types.ContractClass, error)
}

type StateManager interface {
	trie.TrieManager
	GetContractState(*felt.Felt) (*ContractState, error)
	PutContractState(*ContractState) error
	PutContract(*felt.Felt, *types.Contract) error
	GetContract(*felt.Felt) (*types.Contract, error)
}

type state struct {
	manager   StateManager
	stateTrie trie.Trie
}

func New(manager StateManager, root *felt.Felt) State {
	return &state{
		manager,
		trie.New(manager, root, StateTrieHeight),
	}
}

func (st *state) Root() *felt.Felt {
	return st.stateTrie.Root()
}

func (st *state) GetContractState(address *felt.Felt) (*ContractState, error) {
	leaf, err := st.stateTrie.Get(address)
	if err != nil {
		return nil, err
	}
	if leaf.Cmp(trie.EmptyNode.Bottom()) == 0 {
		return &ContractState{new(felt.Felt), new(felt.Felt)}, nil
	}
	return st.manager.GetContractState(leaf)
}

// notest
func (st *state) GetContract(address *felt.Felt) (*types.Contract, error) {
	contractState, err := st.GetContractState(address)
	if err != nil {
		return nil, err
	}
	return st.manager.GetContract(contractState.ContractHash)
}

func (st *state) SetContract(address *felt.Felt, hash *felt.Felt, code *types.Contract) error {
	contract, err := st.GetContractState(address)
	if err != nil {
		return err
	}
	contract.ContractHash = hash
	if err = st.manager.PutContractState(contract); err != nil {
		return err
	}
	if err = st.manager.PutContract(hash, code); err != nil {
		return err
	}
	return st.stateTrie.Put(address, contract.Hash())
}

// notest
func (st *state) GetSlot(address *felt.Felt, slot *felt.Felt) (*felt.Felt, error) {
	contract, err := st.GetContractState(address)
	if err != nil {
		return nil, err
	}
	storage := trie.New(st.manager, contract.StorageRoot, StorageTrieHeight)
	return storage.Get(slot)
}

func (st *state) SetSlot(address *felt.Felt, slot *felt.Felt, value *felt.Felt) error {
	contract, err := st.GetContractState(address)
	if err != nil {
		return err
	}
	storage := trie.New(st.manager, contract.StorageRoot, StorageTrieHeight)
	if err := storage.Put(slot, value); err != nil {
		return err
	}
	contract.StorageRoot = storage.Root()
	err = st.manager.PutContractState(contract)
	if err != nil {
		return err
	}
	return st.stateTrie.Put(address, contract.Hash())
}

// notest
func (st *state) GetClassHash(address *felt.Felt) (*felt.Felt, error) {
	contract, err := st.GetContractState(address)
	if err != nil {
		return nil, err
	}
	return contract.ContractHash, nil
}

func (st *state) GetClass(blockId any, classHash *felt.Felt) (*types.ContractClass, error) {
	contract, err := st.manager.GetContract(classHash)
	if err != nil {
		return nil, err
	}

	fullDef := contract.FullDef
	var fullDefMap map[string]interface{}
	if err := json.Unmarshal(fullDef, &fullDefMap); err != nil {
		return nil, err
	}
	program := fullDefMap["program"]
	entryPointsByType := fullDefMap["entry_points_by_type"]
	contractClass := &types.ContractClass{
		Program:           program,
		EntryPointsByType: entryPointsByType,
	}

	return contractClass, nil
}

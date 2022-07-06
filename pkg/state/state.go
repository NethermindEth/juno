package state

import (
	"github.com/NethermindEth/juno/pkg/trie"
	"github.com/NethermindEth/juno/pkg/types"
)

const (
	StateTrieHeight   = 251
	StorageTrieHeight = 251
)

type State interface {
	Root() *types.Felt
	GetContract(*types.Felt) (*ContractState, error)
	SetContractHash(*types.Felt, *types.Felt) error
	GetSlot(*types.Felt, *types.Felt) (*types.Felt, error)
	SetSlot(*types.Felt, *types.Felt, *types.Felt) error
}

type StateManager interface {
	trie.TrieManager
	GetContractState(*types.Felt) (*ContractState, error)
	PutContractState(*ContractState) error
}

type state struct {
	manager   StateManager
	stateTrie trie.Trie
}

func New(manager StateManager, root *types.Felt) State {
	return &state{manager, trie.New(manager, root, StateTrieHeight)}
}

func (st *state) Root() *types.Felt {
	return st.stateTrie.Root()
}

func (st *state) GetContract(address *types.Felt) (*ContractState, error) {
	leaf, err := st.stateTrie.Get(address)
	if err != nil {
		return nil, err
	}
	if leaf.Cmp(trie.EmptyNode.Bottom()) == 0 {
		return &ContractState{&types.Felt0, &types.Felt0}, nil
	}
	return st.manager.GetContractState(leaf)
}

func (st *state) SetContractHash(address *types.Felt, hash *types.Felt) error {
	contract, err := st.GetContract(address)
	if err != nil {
		return err
	}
	contract.ContractHash = hash
	err = st.manager.PutContractState(contract)
	if err != nil {
		return err
	}
	return st.stateTrie.Put(address, contract.Hash())
}

func (st *state) GetSlot(address *types.Felt, slot *types.Felt) (*types.Felt, error) {
	contract, err := st.GetContract(address)
	if err != nil {
		return nil, err
	}
	storage := trie.New(st.manager, contract.StorageRoot, StorageTrieHeight)
	return storage.Get(slot)
}

func (st *state) SetSlot(address *types.Felt, slot *types.Felt, value *types.Felt) error {
	contract, err := st.GetContract(address)
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

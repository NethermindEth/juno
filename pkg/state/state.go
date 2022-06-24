package state

import (
	"github.com/NethermindEth/juno/pkg/store"
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

type state struct {
	store     stateStore
	stateTrie trie.Trie
}

func New(store store.KVStorer, root *types.Felt) State {
	stateTrie := trie.New(store, root, StateTrieHeight)
	return &state{stateStore{store}, stateTrie}
}

func (st *state) Root() *types.Felt {
	return st.stateTrie.Root()
}

func (st *state) GetContract(address *types.Felt) (*ContractState, error) {
	leaf, err := st.stateTrie.Get(address)
	if err != nil {
		return nil, err
	}
	if leaf.Cmp(&types.Felt0) == 0 {
		return &ContractState{&types.Felt0, &types.Felt0}, nil
	}
	return st.store.retrieveContract(leaf)
}

func (st *state) SetContractHash(address *types.Felt, hash *types.Felt) error {
	contract, err := st.GetContract(address)
	if err != nil {
		return err
	}
	contract.ContractHash = hash
	st.store.storeContract(contract)
	return st.stateTrie.Put(address, contract.Hash())
}

func (st *state) GetSlot(address *types.Felt, slot *types.Felt) (*types.Felt, error) {
	contract, err := st.GetContract(address)
	if err != nil {
		return nil, err
	}
	storage := trie.New(st.store, contract.StorageRoot, StorageTrieHeight)
	return storage.Get(slot)
}

func (st *state) SetSlot(address *types.Felt, slot *types.Felt, value *types.Felt) error {
	contract, err := st.GetContract(address)
	if err != nil {
		return err
	}
	storage := trie.New(st.store, contract.StorageRoot, StorageTrieHeight)
	if err := storage.Put(slot, value); err != nil {
		return err
	}
	contract.StorageRoot = storage.Root()
	st.store.storeContract(contract)
	return st.stateTrie.Put(address, contract.Hash())
}

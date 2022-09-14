package state

import (
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/trie"
)

const (
	StateTrieHeight   = 251
	StorageTrieHeight = 251
)

type State interface {
	Root() *felt.Felt
	GetContractState(address *felt.Felt) (*ContractState, error)
	InitNewContract(address *felt.Felt, classHash *felt.Felt) error
	GetSlot(address *felt.Felt, slot *felt.Felt) (*felt.Felt, error)
	SetSlots(address *felt.Felt, slots []Slot) error
	GetClassHash(address *felt.Felt) (*felt.Felt, error)
}

type StateManager interface {
	trie.TrieManager
	GetContractState(*felt.Felt) (*ContractState, error)
	PutContractState(*ContractState) error
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

func (st *state) InitNewContract(address *felt.Felt, classHash *felt.Felt) error {
	contractState := ContractState{
		ContractHash: classHash,
		StorageRoot:  new(felt.Felt),
	}
	if err := st.manager.PutContractState(&contractState); err != nil {
		return err
	}
	return st.stateTrie.Put(address, contractState.Hash())
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

func (st *state) SetSlots(address *felt.Felt, slots []Slot) error {
	contract, err := st.GetContractState(address)
	if err != nil {
		return err
	}
	storage := trie.New(st.manager, contract.StorageRoot, StorageTrieHeight)
	for _, slot := range slots {
		if err := storage.Put(slot.Key, slot.Value); err != nil {
			return err
		}
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

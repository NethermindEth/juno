package state

import (
	"errors"

	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/types"
)

// ErrContractNotFound is returned when the contract is not found in the state.
var ErrContractNotFound = errors.New("contract not found")

type stateStore struct {
	store.KVStorer
}

func (kvs *stateStore) storeContract(contract *ContractState) error {
	marshaled, err := contract.MarshalBinary()
	if err != nil {
		return err
	}
	kvs.Put(contract.Hash().Bytes(), marshaled)
	return nil
}

func (kvs *stateStore) retrieveContract(hash *types.Felt) (*ContractState, error) {
	marshaled, ok := kvs.Get(append([]byte("contract_state:"), hash.Bytes()...))
	if !ok {
		return nil, ErrContractNotFound
	}
	contract := &ContractState{}
	if err := contract.UnmarshalBinary(marshaled); err != nil {
		return nil, err
	}
	return contract, nil
}

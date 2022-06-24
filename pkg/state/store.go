package state

import (
	"errors"

	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/types"
)

type stateStore struct {
	store.KVStorer
}

func (kvs *stateStore) storeContract(contract *ContractState) error {
	marshaled, err := contract.MarshalBinary()
	if err != nil {
		return err
	}
	kvs.Put(append([]byte("contract_state:"), contract.Hash().Bytes()...), marshaled)
	return nil
}

func (kvs *stateStore) retrieveContract(hash *types.Felt) (*ContractState, error) {
	marshaled, ok := kvs.Get(append([]byte("contract_state:"), hash.Bytes()...))
	if !ok {
		return nil, errors.New("contract not found")
	}
	contract := &ContractState{}
	if err := contract.UnmarshalBinary(marshaled); err != nil {
		return nil, err
	}
	return contract, nil
}

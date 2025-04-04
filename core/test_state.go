package core

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type testState struct {
	*State
}

func NewTestState(txn db.Transaction) *testState {
	return &testState{
		State: NewState(txn),
	}
}

func (s *testState) RootWithStateUpdate(
	blockNumber uint64, update *StateUpdate, declaredClasses map[felt.Felt]Class,
) (*felt.Felt, error) {
	err := s.processStateDiff(blockNumber, update, declaredClasses)
	if err != nil {
		return nil, err
	}

	root, err := s.Root()
	if err != nil {
		return nil, err
	}

	update.NewRoot = root

	err = s.Revert(blockNumber, update)
	if err != nil {
		return nil, err
	}

	return root, nil
}

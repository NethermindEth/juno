package core

import (
	"github.com/NethermindEth/juno/core/felt"
)

type PreConfirmed struct {
	Block       *Block
	StateUpdate *StateUpdate
	// Node does not fetch unknown classes. but we keep it for sequencer
	NewClasses            map[felt.Felt]Class
	TransactionStateDiffs []*StateDiff
	CandidateTxs          []Transaction
}

func NewPreConfirmed(block *Block, stateUpdate *StateUpdate, transactionStateDiffs []*StateDiff, candidateTxs []Transaction) PreConfirmed {
	return PreConfirmed{
		Block:                 block,
		StateUpdate:           stateUpdate,
		NewClasses:            make(map[felt.Felt]Class),
		TransactionStateDiffs: transactionStateDiffs,
		CandidateTxs:          candidateTxs,
	}
}

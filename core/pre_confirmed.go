package core

import (
	"github.com/NethermindEth/juno/core/felt"
)

type PreConfirmed struct {
	Block                 *Block
	StateUpdate           *StateUpdate
	TransactionStateDiffs []*StateDiff
	CandidateTxs          []Transaction
	NewClasses            map[felt.Felt]Class
}

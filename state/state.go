package state

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
)

type StateReader interface {
	Class(hash *felt.Felt) (*core.Class, error)
	Contract(address *felt.Felt) (*core.Contract, error)
	Slot(address *felt.Felt, key *felt.Felt) (*felt.Felt, error)

	Root() *felt.Felt
}

type StateWriter interface {
	PutClass(hash *felt.Felt, c *core.Class) error
	PutContract(address *felt.Felt, c *core.Contract) error
	PutSlot(address *felt.Felt, key *felt.Felt, value *felt.Felt) error

	CalculateRoot() (*felt.Felt, error)

	ApplyDiff(stateDiff *core.StateDiff) error
	ApplyReverseDiff(stateDiff *core.StateDiff) error
}

type State struct {
	state        db.Bucket
	contractTrie db.Bucket
	storageTrie  db.Bucket
}

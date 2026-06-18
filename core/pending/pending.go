package pending

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

var (
	ErrPreConfirmedNotFound        = errors.New("pre_confirmed not found")
	ErrTransactionNotFound         = errors.New("pre_confirmed: transaction not found")
	ErrTransactionReceiptNotFound  = errors.New("pre_confirmed: transaction receipt not found")
	ErrTransactionIndexOutOfBounds = errors.New(
		"pre_confirmed: transaction index out of bounds",
	)
)

// Deprecated: Pending is the pre-0.14.0 pending block variant. It is retained solely as a
// placeholder returned by rpc/v6/v8's Pending() and MakeEmptyPendingForParent to satisfy
// the "pending" block ID in the v6/v8 RPC spec. Remove this type when rpc/v6/v8 are deprecated.
type Pending struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
	NewClasses  map[felt.Felt]core.ClassDefinition
}

// Deprecated: NewPending constructs the deprecated Pending type.
func NewPending(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) Pending {
	return Pending{
		Block:       block,
		StateUpdate: stateUpdate,
		NewClasses:  newClasses,
	}
}

func (p *Pending) GetBlock() *core.Block {
	return p.Block
}

func (p *Pending) GetHeader() *core.Header {
	return p.Block.Header
}

func (p *Pending) GetTransactions() []core.Transaction {
	return p.Block.Transactions
}

func (p *Pending) GetStateUpdate() *core.StateUpdate {
	return p.StateUpdate
}

type PreConfirmed struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
	// Node does not fetch unknown classes. but we keep it for sequencer
	NewClasses            map[felt.Felt]core.ClassDefinition
	TransactionStateDiffs []*core.StateDiff
	// BlockIdentifier is an identifier returned by the feeder gateway
	// that uniquely identifies the current round of the pre_confirmed block.
	// It is used to negotiate delta-sync responses on subsequent polls.
	BlockIdentifier string
}

func NewPreConfirmed(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	transactionStateDiffs []*core.StateDiff,
	blockIdentifier string,
) PreConfirmed {
	return PreConfirmed{
		Block:                 block,
		StateUpdate:           stateUpdate,
		TransactionStateDiffs: transactionStateDiffs,
		BlockIdentifier:       blockIdentifier,
	}
}

func (p *PreConfirmed) WithNewClasses(newClasses map[felt.Felt]core.ClassDefinition) *PreConfirmed {
	p.NewClasses = newClasses
	return p
}

func (p *PreConfirmed) Copy() *PreConfirmed {
	cp := *p // shallow copy of the struct
	return &cp
}

func (p *PreConfirmed) GetBlock() *core.Block {
	return p.Block
}

func (p *PreConfirmed) GetHeader() *core.Header {
	return p.Block.Header
}

func (p *PreConfirmed) GetTransactions() []core.Transaction {
	return p.Block.Transactions
}

func (p *PreConfirmed) GetStateUpdate() *core.StateUpdate {
	return p.StateUpdate
}

func (p *PreConfirmed) GetNewClasses() map[felt.Felt]core.ClassDefinition {
	return p.NewClasses
}

func (p *PreConfirmed) GetTransactionStateDiffs() []*core.StateDiff {
	return p.TransactionStateDiffs
}

// TransactionByHash locates a transaction by hash in the block and returns
// it together with its index. Returns ErrTransactionNotFound when missing.
func (p *PreConfirmed) TransactionByHash(hash *felt.Felt) (core.Transaction, uint, error) {
	for i, tx := range p.Block.Transactions {
		if tx.Hash().Equal(hash) {
			return tx, uint(i), nil // TODO(Ege): maybe use uint16
		}
	}
	return nil, 0, ErrTransactionNotFound
}

func (p *PreConfirmed) ReceiptByHash(
	hash *felt.Felt,
) (*core.TransactionReceipt, error) {
	for _, receipt := range p.Block.Receipts {
		if receipt.TransactionHash.Equal(hash) {
			return receipt, nil
		}
	}

	return nil, ErrTransactionReceiptNotFound
}

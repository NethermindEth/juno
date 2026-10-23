package pending

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

// NewPreConfirmedWithBloom pairs a pre_confirmed with its event bloom filter,
// built from the pre_confirmed block's receipts.
func NewPreConfirmedWithBloom(preConfirmed *PreConfirmed) core.WithBloom[*PreConfirmed] {
	return core.WithBloom[*PreConfirmed]{
		Value: preConfirmed,
		Bloom: core.EventsBloom(preConfirmed.Block.Receipts),
	}
}

var (
	ErrPreConfirmedNotFound        = errors.New("pre_confirmed not found")
	ErrTransactionNotFound         = errors.New("pre_confirmed: transaction not found")
	ErrTransactionReceiptNotFound  = errors.New("pre_confirmed: transaction receipt not found")
	ErrTransactionIndexOutOfBounds = errors.New(
		"pre_confirmed: transaction index out of bounds",
	)
)

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
func (p *PreConfirmed) TransactionByHash(
	hash *felt.TransactionHash,
) (core.Transaction, uint, error) {
	for i, tx := range p.Block.Transactions {
		if tx.Hash().Equal((*felt.Felt)(hash)) {
			return tx, uint(i), nil
		}
	}
	return nil, 0, ErrTransactionNotFound
}

func (p *PreConfirmed) ReceiptByHash(
	hash *felt.TransactionHash,
) (*core.TransactionReceipt, error) {
	for _, receipt := range p.Block.Receipts {
		if receipt.TransactionHash.Equal((*felt.Felt)(hash)) {
			return receipt, nil
		}
	}

	return nil, ErrTransactionReceiptNotFound
}

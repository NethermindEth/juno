package core

import (
	"errors"

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
// placeholder returned by rpc/v8's PendingData() and MakeEmptyPendingForParent to satisfy
// the "pending" block ID in the v8 RPC spec. Remove this type when rpc/v8 is deprecated.
type Pending struct {
	Block       *Block
	StateUpdate *StateUpdate
	NewClasses  map[felt.Felt]ClassDefinition
}

// Deprecated: NewPending constructs the deprecated Pending type.
func NewPending(
	block *Block,
	stateUpdate *StateUpdate,
	newClasses map[felt.Felt]ClassDefinition,
) Pending {
	return Pending{
		Block:       block,
		StateUpdate: stateUpdate,
		NewClasses:  newClasses,
	}
}

func (p *Pending) GetBlock() *Block {
	return p.Block
}

func (p *Pending) GetHeader() *Header {
	return p.Block.Header
}

func (p *Pending) GetTransactions() []Transaction {
	return p.Block.Transactions
}

func (p *Pending) GetStateUpdate() *StateUpdate {
	return p.StateUpdate
}

type PreLatest Pending

type PreConfirmed struct {
	Block       *Block
	StateUpdate *StateUpdate
	// Node does not fetch unknown classes. but we keep it for sequencer
	NewClasses            map[felt.Felt]ClassDefinition
	TransactionStateDiffs []*StateDiff
	CandidateTxs          []Transaction
	// Optional field, exists if pre_confirmed is N+2 when latest is N
	PreLatest *PreLatest
}

func NewPreConfirmed(
	block *Block,
	stateUpdate *StateUpdate,
	transactionStateDiffs []*StateDiff,
	candidateTxs []Transaction,
) PreConfirmed {
	return PreConfirmed{
		Block:                 block,
		StateUpdate:           stateUpdate,
		TransactionStateDiffs: transactionStateDiffs,
		CandidateTxs:          candidateTxs,
	}
}

func (p *PreConfirmed) WithNewClasses(newClasses map[felt.Felt]ClassDefinition) *PreConfirmed {
	p.NewClasses = newClasses
	return p
}

func (p *PreConfirmed) WithPreLatest(preLatest *PreLatest) *PreConfirmed {
	p.PreLatest = preLatest
	return p
}

func (p *PreConfirmed) Copy() *PreConfirmed {
	cp := *p // shallow copy of the struct
	return &cp
}

func (p *PreConfirmed) GetBlock() *Block {
	return p.Block
}

func (p *PreConfirmed) GetHeader() *Header {
	return p.Block.Header
}

func (p *PreConfirmed) GetTransactions() []Transaction {
	return p.Block.Transactions
}

func (p *PreConfirmed) GetStateUpdate() *StateUpdate {
	return p.StateUpdate
}

func (p *PreConfirmed) GetNewClasses() map[felt.Felt]ClassDefinition {
	return p.NewClasses
}

func (p *PreConfirmed) GetCandidateTransaction() []Transaction {
	return p.CandidateTxs
}

func (p *PreConfirmed) GetTransactionStateDiffs() []*StateDiff {
	return p.TransactionStateDiffs
}

func (p *PreConfirmed) GetPreLatest() *PreLatest {
	return p.PreLatest
}

func (p *PreConfirmed) Validate(parent *Header) bool {
	if parent == nil {
		return p.Block.Number == 0
	}

	if p.Block.Number == parent.Number+1 {
		// preconfirmed is latest + 1
		return true
	}

	if p.PreLatest == nil {
		return false
	}

	// is pre_confirmed based on valid pre_latest
	return p.Block.Number == p.PreLatest.Block.Number+1 &&
		p.PreLatest.Block.ParentHash.Equal(parent.Hash)
}

func (p *PreConfirmed) TransactionByHash(hash *felt.Felt) (Transaction, error) {
	if preLatest := p.PreLatest; preLatest != nil {
		for _, tx := range preLatest.Block.Transactions {
			if tx.Hash().Equal(hash) {
				return tx, nil
			}
		}
	}

	for _, tx := range p.CandidateTxs {
		if tx.Hash().Equal(hash) {
			return tx, nil
		}
	}

	for _, tx := range p.Block.Transactions {
		if tx.Hash().Equal(hash) {
			return tx, nil
		}
	}

	return nil, ErrTransactionNotFound
}

func (p *PreConfirmed) ReceiptByHash(
	hash *felt.Felt,
) (*TransactionReceipt, *felt.Felt, uint64, error) {
	if preLatest := p.PreLatest; preLatest != nil {
		for _, receipt := range preLatest.Block.Receipts {
			if receipt.TransactionHash.Equal(hash) {
				return receipt, preLatest.Block.Header.ParentHash, preLatest.Block.Number, nil
			}
		}
	}

	for _, receipt := range p.Block.Receipts {
		if receipt.TransactionHash.Equal(hash) {
			return receipt, nil, p.Block.Number, nil
		}
	}

	return nil, nil, 0, ErrTransactionReceiptNotFound
}

func (p *PreConfirmed) PendingStateBeforeIndex(
	baseState StateReader,
	index uint,
) (StateReader, error) {
	if index > uint(len(p.Block.Transactions)) {
		return nil, ErrTransactionIndexOutOfBounds
	}

	stateDiff := EmptyStateDiff()
	newClasses := make(map[felt.Felt]ClassDefinition)

	// Add pre_latest state diff if available
	preLatest := p.PreLatest
	if preLatest != nil {
		stateDiff.Merge(preLatest.StateUpdate.StateDiff)
		newClasses = preLatest.NewClasses
	}

	// Apply transaction state diffs up to the given index
	txStateDiffs := p.TransactionStateDiffs
	for _, txStateDiff := range txStateDiffs[:index] {
		stateDiff.Merge(txStateDiff)
	}

	return NewPendingState(&stateDiff, newClasses, baseState, p.Block.Number), nil
}

func (p *PreConfirmed) PendingState(baseState StateReader) StateReader {
	stateDiff := EmptyStateDiff()
	newClasses := make(map[felt.Felt]ClassDefinition)

	// Add pre_latest state diff if available
	preLatest := p.PreLatest
	if preLatest != nil {
		stateDiff.Merge(preLatest.StateUpdate.StateDiff)
		newClasses = preLatest.NewClasses
	}

	stateDiff.Merge(p.StateUpdate.StateDiff)

	return NewPendingState(&stateDiff, newClasses, baseState, p.Block.Number)
}

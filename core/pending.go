package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
)

var (
	_ PendingData = (*Pending)(nil)
	_ PendingData = (*PreConfirmed)(nil)
)

var (
	ErrPendingDataNotFound         = errors.New("pending_data not found")
	ErrTransactionNotFound         = errors.New("pending_data: transaction not found")
	ErrTransactionReceiptNotFound  = errors.New("pending_data: transaction receipt not found")
	ErrTransactionIndexOutOfBounds = errors.New(
		"pending_data: transaction index out of bounds",
	)
	ErrPendingStateBeforeIndexNotSupported = errors.New(
		"pending_data: PendingStateBeforeIndex not supported for Pending block",
	)
)

type PendingDataVariant uint8

const (
	PendingBlockVariant PendingDataVariant = iota + 1
	PreConfirmedBlockVariant
)

type PendingData interface {
	GetBlock() *Block
	GetHeader() *Header
	GetTransactions() []Transaction
	GetStateUpdate() *StateUpdate
	GetNewClasses() map[felt.Felt]ClassDefinition
	GetCandidateTransaction() []Transaction
	GetTransactionStateDiffs() []*StateDiff
	GetPreLatest() *PreLatest
	// Validate returns true if pendingData is valid for given predecessor,
	// otherwise returns false
	Validate(parent *Header) bool
	Variant() PendingDataVariant
	TransactionByHash(hash *felt.Felt) (Transaction, error)
	ReceiptByHash(hash *felt.Felt) (*TransactionReceipt, *felt.Felt, uint64, error)
	// PendingStateBeforeIndex returns the state obtained by applying all transaction state diffs
	// up to given index in the pre-confirmed block.
	PendingStateBeforeIndex(baseState StateReader, index uint) (StateReader, error)
	// PendingState returns the state resulting from execution of the pending data
	PendingState(baseState StateReader) StateReader
}

type Pending struct {
	Block       *Block
	StateUpdate *StateUpdate
	NewClasses  map[felt.Felt]ClassDefinition
}

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

func (p *Pending) GetNewClasses() map[felt.Felt]ClassDefinition {
	return p.NewClasses
}

func (p *Pending) GetCandidateTransaction() []Transaction {
	return []Transaction{}
}

func (p *Pending) GetTransactionStateDiffs() []*StateDiff {
	return []*StateDiff{}
}

func (p *Pending) GetPreLatest() *PreLatest {
	return nil
}

func (p *Pending) Variant() PendingDataVariant {
	return PendingBlockVariant
}

func (p *Pending) Validate(parent *Header) bool {
	if parent == nil {
		return p.Block.ParentHash.Equal(&felt.Zero)
	}

	return p.Block.ParentHash.Equal(parent.Hash)
}

func (p *Pending) TransactionByHash(hash *felt.Felt) (Transaction, error) {
	for _, tx := range p.Block.Transactions {
		if tx.Hash().Equal(hash) {
			return tx, nil
		}
	}

	return nil, ErrTransactionNotFound
}

func (p *Pending) ReceiptByHash(
	hash *felt.Felt,
) (*TransactionReceipt, *felt.Felt, uint64, error) {
	for _, receipt := range p.Block.Receipts {
		if receipt.TransactionHash.Equal(hash) {
			return receipt, p.Block.ParentHash, p.Block.Number, nil
		}
	}

	return nil, nil, 0, ErrTransactionReceiptNotFound
}

func (p *Pending) PendingStateBeforeIndex(
	baseState StateReader,
	index uint,
) (StateReader, error) {
	return nil, ErrPendingStateBeforeIndexNotSupported
}

func (p *Pending) PendingState(baseState StateReader) StateReader {
	return NewPendingState(
		p.StateUpdate.StateDiff,
		p.NewClasses,
		baseState,
	)
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

func (p *PreConfirmed) Variant() PendingDataVariant {
	return PreConfirmedBlockVariant
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

	return NewPendingState(&stateDiff, newClasses, baseState), nil
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

	return NewPendingState(&stateDiff, newClasses, baseState)
}

package core

import (
	"github.com/NethermindEth/juno/core/felt"
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
	GetNewClasses() map[felt.Felt]Class
	GetCandidateTransaction() []Transaction
	GetTransactionStateDiffs() []*StateDiff
	GetPreLatest() *PreLatest
	// Validate returns true if pendingData is valid for given predecessor,
	// otherwise returns false
	Validate(parent *Header) bool
	Variant() PendingDataVariant
}

type Pending struct {
	Block       *Block
	StateUpdate *StateUpdate
	NewClasses  map[felt.Felt]Class
}

func NewPending(block *Block, stateUpdate *StateUpdate, newClasses map[felt.Felt]Class) Pending {
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

func (p *Pending) GetNewClasses() map[felt.Felt]Class {
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

type PreLatest Pending

type PreConfirmed struct {
	Block       *Block
	StateUpdate *StateUpdate
	// Node does not fetch unknown classes. but we keep it for sequencer
	NewClasses            map[felt.Felt]Class
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

func (p *PreConfirmed) WithNewClasses(newClasses map[felt.Felt]Class) {
	p.NewClasses = newClasses
}

func (p *PreConfirmed) WithPreLatest(preLatest *PreLatest) {
	p.PreLatest = preLatest
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

func (p *PreConfirmed) GetNewClasses() map[felt.Felt]Class {
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

	preLatestNumber := p.PreLatest.Block.Number

	if p.Block.Number == preLatestNumber+1 &&
		preLatestNumber == parent.Number+1 {
		// preconfirmed is latest + 2
		return true
	}

	return false
}

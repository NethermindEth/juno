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
	Variant() PendingDataVariant
}

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

func (p *PreConfirmed) Variant() PendingDataVariant {
	return PreConfirmedBlockVariant
}

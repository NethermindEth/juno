package core

import (
	"github.com/NethermindEth/juno/core/felt"
)

type PendingDataVariant uint8

const (
	PendingBlockVariant PendingDataVariant = iota + 1
	PreConfirmedBlockVariant
)

type PendingDataInterface interface {
	GetBlock() *Block
	GetHeader() *Header
	GetTransactions() []Transaction
	GetStateUpdate() *StateUpdate
	GetNewClasses() map[felt.Felt]Class
	GetCandidateTransaction() []Transaction
	Variant() PendingDataVariant
}

type PendingData struct {
	data PendingDataInterface
}

func NewPendingData(data PendingDataInterface) PendingData {
	return PendingData{
		data: data,
	}
}

func (p *PendingData) GetBlock() *Block {
	return p.data.GetBlock()
}

func (p *PendingData) GetHeader() *Header {
	return p.data.GetHeader()
}

func (p *PendingData) GetTransactions() []Transaction {
	return p.data.GetTransactions()
}

func (p *PendingData) GetStateUpdate() *StateUpdate {
	return p.data.GetStateUpdate()
}

func (p *PendingData) GetNewClasses() map[felt.Felt]Class {
	return p.data.GetNewClasses()
}

func (p *PendingData) GetCandidateTransaction() []Transaction {
	return p.data.GetCandidateTransaction()
}

func (p *PendingData) Variant() PendingDataVariant {
	return p.data.Variant()
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

func (p *PreConfirmed) AsPendingData() PendingData {
	return PendingData{data: p}
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

func (p *PreConfirmed) Variant() PendingDataVariant {
	return PreConfirmedBlockVariant
}

package rpc

import (
	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/transaction"
	"github.com/NethermindEth/juno/pkg/types"
)

func (x *BlockResponse) fromDatabaseBlock(y *block.Block) {
	x.BlockHash = types.BytesToFelt(y.Hash)
	x.ParentHash = types.BytesToFelt(y.ParentBlockHash)
	x.BlockNumber = y.BlockNumber
	x.Status = BlockStatus(y.Status)
	x.Sequencer = types.BytesToFelt(y.SequencerAddress)
	x.NewRoot = types.BytesToFelt(y.GlobalStateRoot)
	x.OldRoot = types.BytesToFelt(y.OldRoot)
	x.AcceptedTime = uint64(y.AcceptedTime)
}

func (x *Txn) fromDatabase(y *transaction.Transaction) {
	x.TxnHash = types.BytesToFelt(y.Hash)

	if tx := y.GetInvoke(); tx != nil {
		x.CallData = make([]types.Felt, len(tx.CallData))
		for i, data := range tx.CallData {
			x.CallData[i] = types.BytesToFelt(data)
		}
		x.EntryPointSelector = types.BytesToFelt(tx.EntryPointSelector)
		x.ContractAddress = types.BytesToFelt(tx.ContractAddress)
		x.MaxFee = types.BytesToFelt(tx.MaxFee)
		return
	}
}

func (x *TxnReceipt) fromDatabase(y *transaction.TransactionReceipt) {
	x.TxnHash = types.BytesToFelt(y.TxHash)
	x.Status = y.Status.String()
	x.StatusData = y.StatusData
	x.MessagesSent = make([]MsgToL1, len(y.MessagesSent))
	for i, msg := range y.MessagesSent {
		x.MessagesSent[i].fromDatabase(msg)
	}
	x.L1OriginMessage.fromDatabase(y.L1OriginMessage)
	x.Events = make([]Event, len(y.Events))
	for i, e := range y.Events {
		x.Events[i].fromDatabase(e)
	}
}

func (x *MsgToL1) fromDatabase(y *transaction.MessageToL1) {
	x.ToAddress = types.BytesToFelt(y.ToAddress)
	x.Payload = make([]types.Felt, len(y.Payload))
	for i, p := range y.Payload {
		x.Payload[i] = types.BytesToFelt(p)
	}
}

func (x *MsgToL2) fromDatabase(y *transaction.MessageToL2) {
	x.FromAddress = EthAddress(y.FromAddress)
	x.Payload = make([]types.Felt, len(y.Payload))
	for i, p := range y.Payload {
		x.Payload[i] = types.BytesToFelt(p)
	}
}

func (x *Event) fromDatabase(y *transaction.Event) {
	x.FromAddress = types.BytesToFelt(y.FromAddress)
	x.Keys = make([]types.Felt, len(y.Keys))
	for i, k := range y.Keys {
		x.Keys[i] = types.BytesToFelt(k)
	}
	x.Data = make([]types.Felt, len(y.Data))
	for i, d := range y.Data {
		x.Data[i] = types.BytesToFelt(d)
	}
}

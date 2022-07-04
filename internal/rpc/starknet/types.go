package starknet

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/internal/services"
	"github.com/NethermindEth/juno/pkg/common"
	"github.com/NethermindEth/juno/pkg/types"
)

type BlockTag string

const (
	BlocktagLatest  BlockTag = "latest"
	BlocktagPending BlockTag = "pending"
)

var blockTags = []BlockTag{BlocktagLatest, BlocktagPending}

type BlockHashOrTag struct {
	Hash *types.BlockHash
	Tag  *BlockTag
}

func (x *BlockHashOrTag) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewBuffer(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch t := token.(type) {
	case string:
		for _, tag := range blockTags {
			if t == string(tag) {
				*x = BlockHashOrTag{}
				x.Tag = &tag
				return nil
			}
		}
		if common.IsHex(t) {
			hash := types.HexToBlockHash(t)
			*x = BlockHashOrTag{
				Hash: &hash,
			}
		}
	default:
		// notest
		return errors.New("unexpected token type")
	}
	return nil
}

type RequestedScope string

const (
	ScopeTxnHash            RequestedScope = "TXN_HASH"
	ScopeFullTxns           RequestedScope = "FULL_TXNS"
	ScopeFullTxnAndReceipts RequestedScope = "FULL_TXN_AND_RECEIPTS"
)

type ProtocolVersion string

type BlockNumberOrTag struct {
	Number *uint64
	Tag    *BlockTag
}

func (x *BlockNumberOrTag) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewBuffer(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch t := token.(type) {
	case float64:
		blockNumber := uint64(t)
		if blockNumber < 0 {
			// notest
			return errors.New("invalid block number")
		}
		*x = BlockNumberOrTag{Number: &blockNumber}
	case string:
		// notest
		for _, tag := range blockTags {
			if t == string(tag) {
				*x = BlockNumberOrTag{Tag: &tag}
				break
			}
		}
	default:
		// notest
		return errors.New("unexpected token type")
	}
	return nil
}

type BlockResponse struct {
	// A field element of 251 bits. Represented as up to 64 hex digits
	BlockHash types.BlockHash `json:"block_hash"`
	// The hash of this block's parent
	ParentHash types.BlockHash `json:"parent_hash"`
	// The block number (its height)
	BlockNumber uint64 `json:"block_number"`
	// The status of the block
	Status types.BlockStatus `json:"status"`
	// The identity of the sequencer submitting this block
	Sequencer types.Address `json:"sequencer"`
	// The new global state root
	NewRoot types.Felt `json:"new_root"`
	// The previous global state root
	OldRoot types.Felt `json:"old_root"`
	// When the block was accepted on L1. Formatted as...
	AcceptedTime int64 `json:"accepted_time"`
	// Transactions in the Block
	Transactions interface{} `json:"transactions"`
}

func NewBlockResponse(block *types.Block, scope RequestedScope) *BlockResponse {
	response := &BlockResponse{
		BlockHash:    block.BlockHash,
		ParentHash:   block.ParentHash,
		BlockNumber:  block.BlockNumber,
		Status:       block.Status,
		Sequencer:    block.Sequencer,
		NewRoot:      block.NewRoot,
		OldRoot:      block.OldRoot,
		AcceptedTime: block.AcceptedTime,
	}
	switch scope {
	case ScopeTxnHash:
		response.Transactions = block.TxHashes
	case ScopeFullTxns:
		txns := make([]*Txn, len(block.TxHashes))
		for i, txHash := range block.TxHashes {
			transaction := services.TransactionService.GetTransaction(txHash)
			txn := NewTxn(transaction)
			txns[i] = txn
		}
		response.Transactions = txns
	case ScopeFullTxnAndReceipts:
		txns := make([]*TxnAndReceipt, len(block.TxHashes))
		for i, txHash := range block.TxHashes {
			txnAndReceipt := &TxnAndReceipt{}
			transaction := services.TransactionService.GetTransaction(txHash)
			if transaction != nil {
				txn := NewTxn(transaction)
				txnAndReceipt.Txn = *txn
			}
			receipt := services.TransactionService.GetReceipt(txHash)
			if receipt != nil {
				r := NewTxnReceipt(receipt)
				txnAndReceipt.TxnReceipt = *r
			}
			txns[i] = txnAndReceipt
		}
		response.Transactions = txns
	}
	return response
}

type TxnAndReceipt struct {
	Txn
	TxnReceipt
}

func (x *TxnAndReceipt) MarshalJSON() ([]byte, error) {
	type TxnAndReceipt struct {
		TxnHash            types.TransactionHash   `json:"txn_hash"`
		MaxFee             types.Felt              `json:"max_fee"`
		ContractAddress    types.Address           `json:"contract_address"`
		EntryPointSelector types.Felt              `json:"entry_point_selector"`
		CallData           []types.Felt            `json:"call_data"`
		Status             types.TransactionStatus `json:"status,omitempty"`
		StatusData         string                  `json:"status_data,omitempty"`
		MessagesSent       []*MsgToL1              `json:"message_sent,omitempty"`
		L1OriginMessage    *MsgToL2                `json:"l_1_origin_message,omitempty"`
		Events             []*Event                `json:"events,omitempty"`
	}
	var enc TxnAndReceipt
	enc.TxnHash = x.Txn.TxnHash
	enc.MaxFee = x.MaxFee
	enc.ContractAddress = x.ContractAddress
	enc.EntryPointSelector = x.EntryPointSelector
	enc.CallData = x.CallData
	enc.Status = x.Status
	enc.StatusData = x.StatusData
	enc.MessagesSent = x.MessagesSent
	enc.L1OriginMessage = x.L1OriginMessage
	enc.Events = x.Events
	return json.Marshal(enc)
}

// Txn Transaction
type Txn struct {
	// The function the transaction invokes
	FunctionCall
	// The hash identifying the transaction
	TxnHash types.TransactionHash `json:"txn_hash"`
	MaxFee  types.Felt            `json:"max_fee"`
}

// TxnReceipt Receipt of the transaction
type TxnReceipt struct {
	TxnHash         types.TransactionHash   `json:"txn_hash,omitempty"`
	Status          types.TransactionStatus `json:"status,omitempty"`
	StatusData      string                  `json:"status_data,omitempty"`
	MessagesSent    []*MsgToL1              `json:"messages_sent,omitempty"`
	L1OriginMessage *MsgToL2                `json:"l1_origin_message,omitempty"`
	Events          []*Event                `json:"events,omitempty"`
}

func NewTxnReceipt(receipt *types.TransactionReceipt) *TxnReceipt {
	out := &TxnReceipt{
		TxnHash:    receipt.TxHash,
		Status:     receipt.Status,
		StatusData: receipt.StatusData,
	}
	if len(receipt.MessagesSent) != 0 {
		out.MessagesSent = make([]*MsgToL1, len(receipt.MessagesSent))
		for i, msg := range receipt.MessagesSent {
			out.MessagesSent[i] = NewMsgToL1(&msg)
		}
	}
	if receipt.L1OriginMessage != nil {
		out.L1OriginMessage = NewMsgToL2(receipt.L1OriginMessage)
	}
	if len(receipt.Events) != 0 {
		out.Events = make([]*Event, len(receipt.Events))
		for i, e := range receipt.Events {
			out.Events[i] = NewEvent(&e)
		}
	}
	return out
}

func NewTxn(transaction types.IsTransaction) *Txn {
	switch tx := transaction.(type) {
	case *types.TransactionInvoke:
		return &Txn{
			FunctionCall: FunctionCall{
				ContractAddress:    tx.ContractAddress,
				EntryPointSelector: tx.EntryPointSelector,
				CallData:           tx.CallData,
			},
			TxnHash: tx.GetHash(),
			MaxFee:  tx.MaxFee,
		}
	default:
		return &Txn{}
	}
}

// Event represent a StarkNet Event
type Event struct {
	EventContent
	FromAddress types.Address `json:"from_address"`
}

func NewEvent(event *types.Event) *Event {
	return &Event{
		EventContent: EventContent{
			event.Keys,
			event.Data,
		},
		FromAddress: event.FromAddress,
	}
}

// FunctionCall represents the details of the function call
type FunctionCall struct {
	// ContractAddress Is a field element of 251 bits. Represented as up to 64 hex digits
	ContractAddress types.Address `json:"contract_address"`
	// EntryPointSelector Is a field element of 251 bits. Represented as up to 64 hex digits
	EntryPointSelector types.Felt `json:"entry_point_selector"`
	// CallData are the parameters passed to the function
	CallData []types.Felt `json:"calldata"`
}

type MsgToL1 struct {
	// The target L1 address the message is sent to
	ToAddress types.EthAddress `json:"to_address"`
	// The Payload of the message
	Payload []types.Felt `json:"payload"`
}

func NewMsgToL1(msg *types.MessageL2ToL1) *MsgToL1 {
	return &MsgToL1{
		ToAddress: msg.ToAddress,
		Payload:   msg.Payload,
	}
}

type MsgToL2 struct {
	// The originating L1 contract that sent the message
	FromAddress types.EthAddress `json:"from_address"`
	// The Payload of the message. The call data to the L1 handler
	Payload []types.Felt `json:"payload"`
}

func NewMsgToL2(msg *types.MessageL1ToL2) *MsgToL2 {
	return &MsgToL2{
		FromAddress: msg.FromAddress,
		Payload:     msg.Payload,
	}
}

// EventContent represent the content of an Event
type EventContent struct {
	Keys []types.Felt `json:"keys"`
	Data []types.Felt `json:"data"`
}

// CodeResult The code and ABI for the requested contract
type CodeResult struct {
	Bytecode []types.Felt `json:"bytecode"`
	// The ABI of the contract in JSON format. Uses the same structure as EVM ABI
	Abi string `json:"abi"`
}

package rpc

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/pkg/common"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"
)

type RequestedScope string

const (
	ScopeTxnHash            RequestedScope = "TXN_HASH"
	ScopeFullTxns           RequestedScope = "FULL_TXNS"
	ScopeFullTxnAndReceipts RequestedScope = "FULL_TXN_AND_RECEIPTS"
)

const (
	TxnStatusUnknown      TxnStatus = "UNKNOWN"
	TxnStatusReceived     TxnStatus = "RECEIVED"
	TxnStatusPending      TxnStatus = "PENDING"
	TxnStatusAcceptedOnL2 TxnStatus = "ACCEPTED_ON_L2"
	TxnStatusAcceptedOnL1 TxnStatus = "ACCEPTED_ON_L1"
	TxnStatusRejected     TxnStatus = "REJECTED"
)

const (
	Pending      BlockStatus = "PENDING"
	Proven       BlockStatus = "PROVEN"
	AcceptedOnL2 BlockStatus = "ACCEPTED_ON_L2"
	AcceptedOnL1 BlockStatus = "ACCEPTED_ON_L1"
	Rejected     BlockStatus = "REJECTED"
)

var (
	FailedToReceiveTxn     = ResponseError{1, "Failed to write transaction"}
	ContractNotFound       = ResponseError{20, "Contract not found"}
	InvalidMessageSelector = ResponseError{21, "Invalid message selector"}
	InvalidCallData        = ResponseError{22, "Invalid call data"}
	InvalidStorageKey      = ResponseError{23, "Invalid storage key"}
	InvalidBlockHash       = ResponseError{24, "Invalid block hash"}
	InvalidTxnHash         = ResponseError{25, "Invalid transaction hash"}
	InvalidBlockNumber     = ResponseError{26, "Invalid block number"}
	ContractError          = ResponseError{40, "Contract error"}
)

// FunctionCall represents the details of the function call
type FunctionCall struct {
	// ContractAddress Is a field element of 251 bits. Represented as up to 64 hex digits
	ContractAddress *felt.Felt `json:"contract_address"`
	// EntryPointSelector Is a field element of 251 bits. Represented as up to 64 hex digits
	EntryPointSelector *felt.Felt `json:"entry_point_selector"`
	// CallData are the parameters passed to the function
	CallData []*felt.Felt `json:"calldata"`
}

// BlockHash Is a field element of 251 bits. Represented as up to 64 hex digits
type BlockHash string

// BlockTag Is a tag specifying a dynamic reference to a block
type BlockTag string

const (
	BlocktagLatest  BlockTag = "latest"
	BlocktagPending BlockTag = "pending"
)

var blockTags = []BlockTag{BlocktagLatest, BlocktagPending}

// Felt represent aN field element. Represented as up to 63 hex digits and leading 4 bits zeroed.
type Felt string

// BlockNumber The block's number (its height)
type BlockNumber uint64

// ChainID StarkNet chain id, given in hex representation.
type ChainID string

// ProtocolVersion StarkNet protocol version, given in hex representation.
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
		if t < 0 {
			// notest
			return errors.New("invalid block number")
		}
		blockNumber := uint64(t)
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

// BlockHashOrTag The hash (id) of the requested block or a block tag, for the block referencing the state or call the transaction on.
type BlockHashOrTag struct {
	Hash *felt.Felt
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
			hash := new(felt.Felt).SetHex(t)
			*x = BlockHashOrTag{
				Hash: hash,
			}
		}
	default:
		// notest
		return errors.New("unexpected token type")
	}
	return nil
}

// RequestRPC Represent the calls a function in a contract and returns the return value.  Using this call will not create a transaction; hence, will not change the state
type RequestRPC struct {
	Request   FunctionCall `json:"request"`
	BlockHash BlockHash    `json:"block_hash"`
}

// ResultCall Is a field element of 251 bits. Represented as up to 64 hex digits
type ResultCall []string

// BlockTransactionCount represent the number of transactions in the designated block
type BlockTransactionCount struct {
	TransactionCount int
}

// StateDiffItem represent a change in a single storage item
type StateDiffItem struct {
	// The contract address for which the state changed
	Address Felt `json:"address"`
	// The key of the changed value
	Key Felt `json:"key"`
	// THe new value applied to the given address
	Value Felt `json:"value"`
}

// ContractItem represent a new contract added as part of the new state
type ContractItem struct {
	// The address of the contract code
	Address Felt `json:"address"`
	// The hash of the contract code
	ContractHash Felt `json:"contractHash"`
}

// StateDiff represent the change in state applied in this block, given as a mapping of addresses to the new values and/or new contracts
type StateDiff struct {
	StorageDiffs []StateDiffItem `json:"storage_diffs"`
	Contracts    []ContractItem  `json:"contracts"`
}

type StateUpdate struct {
	BlockHash    BlockHash `json:"block_hash"`
	NewRoot      Felt      `json:"new_root"`
	OldRoot      Felt      `json:"old_root"`
	AcceptedTime uint64    `json:"accepted_time"`
	StateDiff    StateDiff `json:"state_diff"`
}

type Address Felt

// Txn Transaction
type Txn struct {
	// The function the transaction invokes
	FunctionCall
	// The hash identifying the transaction
	TxnHash *felt.Felt `json:"txn_hash"`
	MaxFee  *felt.Felt `json:"max_fee"`
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

type TxnAndReceipt struct {
	Txn
	TxnReceipt
}

func (x *TxnAndReceipt) MarshalJSON() ([]byte, error) {
	// notest
	type TxnAndReceipt struct {
		TxnHash            *felt.Felt              `json:"txn_hash"`
		MaxFee             *felt.Felt              `json:"max_fee"`
		ContractAddress    *felt.Felt              `json:"contract_address"`
		EntryPointSelector *felt.Felt              `json:"entry_point_selector"`
		CallData           []*felt.Felt            `json:"call_data"`
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

type TxnStatus string

// EthAddress represent an ethereum address
type EthAddress string

type MsgToL1 struct {
	// The target L1 address the message is sent to
	ToAddress types.EthAddress `json:"to_address"`
	// The Payload of the message
	Payload []*felt.Felt `json:"payload"`
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
	Payload []*felt.Felt `json:"payload"`
}

func NewMsgToL2(msg *types.MessageL1ToL2) *MsgToL2 {
	return &MsgToL2{
		FromAddress: msg.FromAddress,
		Payload:     msg.Payload,
	}
}

// EventFilter represent an event filter/query
type EventFilter struct {
	FromBlock BlockNumber `json:"fromBlock"`
	ToBlock   BlockNumber `json:"toBlock"`
	Address   Address     `json:"address"`
	Keys      []Felt      `json:"keys"`
}

// EventContent represent the content of an Event
type EventContent struct {
	Keys []*felt.Felt `json:"keys"`
	Data []*felt.Felt `json:"data"`
}

// Event represent a StarkNet Event
type Event struct {
	EventContent
	FromAddress *felt.Felt `json:"from_address"`
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

// EmittedEvent Represent Event information decorated with metadata on where it was emitted
type EmittedEvent struct {
	Event
	BlockHash       BlockHash  `json:"block_hash"`
	TransactionHash *felt.Felt `json:"transaction_hash"`
}

// TxnReceipt Receipt of the transaction
type TxnReceipt struct {
	TxnHash         *felt.Felt              `json:"txn_hash,omitempty"`
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

// CodeResult The code and ABI for the requested contract
type CodeResult struct {
	Bytecode []*felt.Felt `json:"bytecode"`
	// The ABI of the contract in JSON format. Uses the same structure as EVM ABI
	Abi string `json:"abi"`
}

// SyncStatus Returns an object about the sync status, or false if the node is not syncing
type SyncStatus struct {
	// The hash of the block from which the sync started
	StartingBlock BlockHash `json:"starting_block"`
	// The hash of the current block being synchronized
	CurrentBlock BlockHash `json:"current_block"`
	// The hash of the estimated the highest block to be synchronized
	HighestBlock BlockHash `json:"highest_block"`
}

// ResultPageRequest A request for a specific page of results
type ResultPageRequest struct {
	PageSize   uint64 `json:"page_size"`
	PageNumber uint64 `json:"page_number"`
}

type BlockStatus string

// ResponseError represent the possible errors of StarkNet RPC API
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Transactions struct{}

type BlockResponse struct {
	// A field element of 251 bits. Represented as up to 64 hex digits
	BlockHash *felt.Felt `json:"block_hash"`
	// The hash of this block's parent
	ParentHash *felt.Felt `json:"parent_hash"`
	// The block number (its height)
	BlockNumber uint64 `json:"block_number"`
	// The status of the block
	Status types.BlockStatus `json:"status"`
	// The identity of the sequencer submitting this block
	Sequencer *felt.Felt `json:"sequencer"`
	// The new global state root
	NewRoot *felt.Felt `json:"new_root"`
	// The previous global state root
	OldRoot *felt.Felt `json:"old_root"`
	// When the block was accepted on L1. Formatted as...
	AcceptedTime int64 `json:"accepted_time"`
	// Transactions in the Block
	Transactions interface{} `json:"transactions"`
}

func NewBlockResponse(block *types.Block, scope RequestedScope) (*BlockResponse, error) {
	// notest
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
			transaction, err := transactionManager.GetTransaction(txHash)
			if err != nil {
				return nil, err
			}
			txn := NewTxn(transaction)
			txns[i] = txn
		}
		response.Transactions = txns
	case ScopeFullTxnAndReceipts:
		txns := make([]*TxnAndReceipt, len(block.TxHashes))
		for i, txHash := range block.TxHashes {
			txnAndReceipt := &TxnAndReceipt{}
			transaction, err := transactionManager.GetTransaction(txHash)
			if err != nil {
				return nil, err
			}
			if transaction != nil {
				txn := NewTxn(transaction)
				txnAndReceipt.Txn = *txn
			}
			receipt, err := transactionManager.GetReceipt(txHash)
			if err != nil {
				return nil, err
			}
			if receipt != nil {
				r := NewTxnReceipt(receipt)
				txnAndReceipt.TxnReceipt = *r
			}
			txns[i] = txnAndReceipt
		}
		response.Transactions = txns
	}
	return response, nil
}

type StarkNetHash struct{}

// EventRequest represent allOf types.EventFilter and types.ResultPageRequest
type EventRequest struct {
	EventFilter
	ResultPageRequest
}

// EmittedEventArray represent a set of emmited events
type EmittedEventArray []EmittedEvent

// EventResponse represent the struct of the response of events
type EventResponse struct {
	EmittedEventArray
	PageNumber uint64 `json:"page_number"`
}

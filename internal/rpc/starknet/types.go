package starknet

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/internal/db/transaction"

	"github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/pkg/felt"
)

var ErrInvalidBlockId = errors.New("invalid block id")

const (
	blockIdHash BlockIdType = iota
	blockIdTag
	blockIdNumber
)

type BlockIdType int

type BlockId struct {
	idType BlockIdType
	value  any
}

func (id *BlockId) hash() (*felt.Felt, bool) {
	h, ok := id.value.(*felt.Felt)
	return h, ok
}

func (id *BlockId) tag() (string, bool) {
	t, ok := id.value.(string)
	return t, ok
}

func (id *BlockId) number() (uint64, bool) {
	n, ok := id.value.(int64)
	return uint64(n), ok
}

func (id *BlockId) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch t := token.(type) {
	case json.Number:
		value, err := t.Int64()
		if err != nil {
			return err
		}
		id.idType = blockIdNumber
		id.value = value
	case string:
		if isBlockTag(t) {
			id.idType = blockIdTag
			id.value = t
			return nil
		}
		if isFelt(t) {
			id.idType = blockIdHash
			id.value = new(felt.Felt).SetHex(t)
			return nil
		}
		return ErrInvalidBlockId
	default:
		return ErrInvalidBlockId
	}
	return nil
}

type BlockBodyWithTxHashes struct {
	Transactions []string `json:"transactions"`
}

type BlockHeader struct {
	BlockHash   string `json:"block_hash"`
	ParentHash  string `json:"parent_hash"`
	BlockNumber uint64 `json:"block_number"`
	NewRoot     string `json:"new_root"`
	Timestamp   int64  `json:"timestamp"`
	Sequencer   string `json:"sequencer_address"`
}

type BlockWithTxHashes struct {
	BlockStatus string `json:"status"`
	BlockHeader
	BlockBodyWithTxHashes
}

func NewBlockWithTxHashes(block *types.Block) *BlockWithTxHashes {
	txnHashes := make([]string, len(block.TxHashes))
	for i, txnHash := range block.TxHashes {
		txnHashes[i] = txnHash.Hex0x()
	}
	return &BlockWithTxHashes{
		BlockStatus: block.Status.String(),
		BlockHeader: BlockHeader{
			BlockHash:   block.BlockHash.Hex0x(),
			ParentHash:  block.ParentHash.Hex0x(),
			BlockNumber: block.BlockNumber,
			NewRoot:     block.NewRoot.Hex0x(),
			Timestamp:   block.TimeStamp,
			Sequencer:   block.Sequencer.Hex0x(),
		},
		BlockBodyWithTxHashes: BlockBodyWithTxHashes{
			Transactions: txnHashes,
		},
	}
}

type BlockBodyWithTxs struct {
	Transactions []Txn `json:"transactions"`
}

type BlockWithTxs struct {
	Status string `json:"status"`
	BlockHeader
	BlockBodyWithTxs
}

func NewBlockWithTxs(block *types.Block, txnManager *transaction.Manager) (*BlockWithTxs, error) {
	txns := make([]Txn, block.TxCount)
	for i, txHash := range block.TxHashes {
		tx, err := txnManager.GetTransaction(txHash)
		if err != nil {
			// XXX: should return an error or panic? In what case we can't get the transaction?
			return nil, err
		}
		if txns[i], err = NewTxn(tx); err != nil {
			// XXX: should return an error or panic? In what case we can't build the transaction?
			return nil, err
		}
	}
	return &BlockWithTxs{
		Status: block.Status.String(),
		BlockHeader: BlockHeader{
			BlockHash:   block.BlockHash.Hex0x(),
			ParentHash:  block.ParentHash.Hex0x(),
			BlockNumber: block.BlockNumber,
			NewRoot:     block.NewRoot.Hex0x(),
			Timestamp:   block.TimeStamp,
			Sequencer:   block.Sequencer.Hex0x(),
		},
		BlockBodyWithTxs: BlockBodyWithTxs{
			Transactions: txns,
		},
	}, nil
}

type Txn interface {
	isTxn()
}

func NewTxn(tx types.IsTransaction) (Txn, error) {
	switch tx := tx.(type) {
	case *types.TransactionInvoke:
		return NewInvokeTxn(tx), nil
	case *types.TransactionDeploy:
		return NewDeployTxn(tx), nil
	case *types.TransactionDeclare:
		return NewDeclareTxn(tx), nil
	default:
		return nil, errors.New("invalid transaction type")
	}
}

type CommonTxnProperties struct {
	TxnHash          string   `json:"txn_hash"`
	MaxFee           string   `json:"max_fee"`
	Version          string   `json:"version"`
	Signature        []string `json:"signature"`
	Nonce            string   `json:"nonce"`
	Type             string   `json:"type"`
}

type FunctionCall struct {
	ContractAddress    string   `json:"contract_address"`
	EntryPointSelector string   `json:"entry_point_selector"`
	Calldata           []string `json:"calldata"`
}

type InvokeTxn struct {
	CommonTxnProperties
	FunctionCall
}

func NewInvokeTxn(txn *types.TransactionInvoke) *InvokeTxn {
	signature := make([]string, len(txn.Signature))
	for i, sig := range txn.Signature {
		signature[i] = sig.Hex0x()
	}
	calldata := make([]string, len(txn.CallData))
	for i, data := range txn.CallData {
		calldata[i] = data.Hex0x()
	}
	return &InvokeTxn{
		CommonTxnProperties: CommonTxnProperties{
			TxnHash:   txn.Hash.Hex0x(),
			MaxFee:    txn.MaxFee.Hex0x(),
			Version:   "0x0", // XXX: hardcoded version for now
			Signature: signature,
			Nonce:     "", // TODO: Manage transaction nonce
			Type:      "INVOKE",
		},
		FunctionCall: FunctionCall{
			ContractAddress:    txn.ContractAddress.Hex0x(),
			EntryPointSelector: txn.EntryPointSelector.Hex0x(),
			Calldata:           calldata,
		},
	}
}

func (*InvokeTxn) isTxn() {}

type DeclareTxn struct {
	CommonTxnProperties
	ClassHash     string `json:"class_hash"`
	SenderAddress string `json:"sender_address"`
}

func NewDeclareTxn(txn *types.TransactionDeclare) *DeclareTxn {
	signature := make([]string, len(txn.Signature))
	for i, sig := range txn.Signature {
		signature[i] = sig.Hex0x()
	}
	return &DeclareTxn{
		CommonTxnProperties: CommonTxnProperties{
			TxnHash:   txn.Hash.Hex0x(),
			MaxFee:    txn.MaxFee.Hex0x(),
			Version:   "0x0", // XXX: hardcoded version for now
			Signature: signature,
			Nonce:     "", // TODO: Manage transaction nonce
			Type:      "DECLARE",
		},
		ClassHash:     txn.ClassHash.Hex0x(),
		SenderAddress: txn.SenderAddress.Hex0x(),
	}
}

func (*DeclareTxn) isTxn() {}

type DeployTxn struct {
	TxnHash             string   `json:"txn_hash"`
	ClassHash           string   `json:"class_hash"`
	ContractAddress     string   `json:"contract_address"`
	ConstructorCalldata []string `json:"constructor_calldata"`
}

func NewDeployTxn(txn *types.TransactionDeploy) *DeployTxn {
	callData := make([]string, len(txn.ConstructorCallData))
	for i, data := range txn.ConstructorCallData {
		callData[i] = data.Hex0x()
	}
	return &DeployTxn{
		TxnHash:             txn.Hash.Hex0x(),
		ClassHash:           txn.ClassHash.Hex0x(),
		ContractAddress:     txn.ContractAddress.Hex0x(),
		ConstructorCalldata: callData,
	}
}

func (*DeployTxn) isTxn() {}

type Receipt interface {
	isReceipt()
}

func NewReceipt(receipt types.TxnReceipt) (Receipt, error) {
	switch receipt := receipt.(type) {
	case *types.TxnInvokeReceipt:
		var messagesSent []*MsgToL1 = nil
		if len(receipt.MessagesSent) > 0 {
			messagesSent = make([]*MsgToL1, len(receipt.MessagesSent))
			for i, msg := range receipt.MessagesSent {
				messagesSent[i] = NewMsgToL1(msg)
			}
		}
		var events []*Event = nil
		if len(receipt.Events) > 0 {
			events = make([]*Event, len(receipt.Events))
			for i, event := range receipt.Events {
				events[i] = NewEvent(event)
			}
		}
		return &InvokeTxReceipt{
			CommonReceiptProperties: CommonReceiptProperties{
				TxnHash:     receipt.TxnHash.Hex0x(),
				ActualFee:   receipt.ActualFee.Hex0x(),
				Status:      receipt.Status.String(),
				StatusData:  receipt.StatusData,
				BlockHash:   receipt.BlockHash.Hex0x(),
				BlockNumber: receipt.BlockNumber,
			},
			MessagesSent:    messagesSent,
			L1OriginMessage: NewMsgToL2(receipt.L1OriginMessage),
			Events:          events,
		}, nil
	case *types.TxnDeployReceipt:
		return &DeployTxReceipt{
			CommonReceiptProperties: CommonReceiptProperties{
				TxnHash:     receipt.TxnHash.Hex0x(),
				ActualFee:   receipt.ActualFee.Hex0x(),
				Status:      receipt.Status.String(),
				StatusData:  receipt.StatusData,
				BlockHash:   receipt.BlockHash.Hex0x(),
				BlockNumber: receipt.BlockNumber,
			},
		}, nil
	case *types.TxnDeclareReceipt:
		return &DeclareTxReceipt{
			CommonReceiptProperties: CommonReceiptProperties{
				TxnHash:     receipt.TxnHash.Hex0x(),
				ActualFee:   receipt.ActualFee.Hex0x(),
				Status:      receipt.Status.String(),
				StatusData:  receipt.StatusData,
				BlockHash:   receipt.BlockHash.Hex0x(),
				BlockNumber: receipt.BlockNumber,
			},
		}, nil
	default:
		return nil, errors.New("invalid receipt type")
	}
}

type CommonReceiptProperties struct {
	TxnHash          string `json:"txn_hash"`
	TransactionIndex uint64 `json:"transaction_index"`
	ActualFee        string `json:"actual_fee"`
	Status           string `json:"status"`
	StatusData       string `json:"status_data"`
	BlockHash        string `json:"block_hash"`
	BlockNumber      uint64 `json:"block_number"`
}

type InvokeTxReceipt struct {
	CommonReceiptProperties
	MessagesSent    []*MsgToL1 `json:"messages_sent"`
	L1OriginMessage *MsgToL2   `json:"l1_origin_message"`
	Events          []*Event   `json:"events"`
}

func (*InvokeTxReceipt) isReceipt() {}

type DeclareTxReceipt struct {
	CommonReceiptProperties
}

func (*DeclareTxReceipt) isReceipt() {}

type DeployTxReceipt struct {
	CommonReceiptProperties
}

func (*DeployTxReceipt) isReceipt() {}

type MsgToL1 struct {
	ToAddress types.EthAddress `json:"to_address"`
	Payload   []string         `json:"payload"`
}

func NewMsgToL1(msg *types.MsgToL1) *MsgToL1 {
	payload := make([]string, len(msg.Payload))
	for i, data := range msg.Payload {
		payload[i] = data.Hex0x()
	}
	return &MsgToL1{
		ToAddress: msg.ToAddress,
		Payload:   payload,
	}
}

type MsgToL2 struct {
	FromAddress types.EthAddress `json:"from_address"`
	Payload     []string         `json:"payload"`
}

func NewMsgToL2(msg *types.MsgToL2) *MsgToL2 {
	if msg == nil {
		return &MsgToL2{}
	}
	payload := make([]string, len(msg.Payload))
	for i, data := range msg.Payload {
		payload[i] = data.Hex0x()
	}
	return &MsgToL2{
		FromAddress: msg.FromAddress,
		Payload:     payload,
	}
}

type Event struct {
	FromAddress string   `json:"from_address"`
	Keys        []string `json:"keys"`
	Data        []string `json:"data"`
}

func NewEvent(event *types.Event) *Event {
	keys := make([]string, len(event.Keys))
	for i, key := range event.Keys {
		keys[i] = key.Hex0x()
	}
	data := make([]string, len(event.Data))
	for i, d := range event.Data {
		data[i] = d.Hex0x()
	}
	return &Event{
		FromAddress: event.FromAddress.Hex0x(),
		Keys:        keys,
		Data:        data,
	}
}

type ContractClass struct {
	Program           string `json:"program"`
	EntryPointsByType struct {
		Constructor []*ContractEntryPoint `json:"CONSTRUCTOR"`
		External    []*ContractEntryPoint `json:"EXTERNAL"`
		L1Handler   []*ContractEntryPoint `json:"L1_HANDLER"`
	}
}

type ContractEntryPoint struct {
	Offset   string `json:"offset"`
	Selector string `json:"selector"`
}

type FeeEstimate struct {
	GasConsumed string `json:"gas_consumed"`
	GasPrice    string `json:"gas_price"`
	OverallFee  string `json:"overall_fee"`
}

type SyncStatus struct {
	StartingBlockHash   string `json:"starting_block_hash"`
	StartingBlockNumber string `json:"starting_block_number"`
	CurrentBlockHash    string `json:"current_block_hash"`
	CurrentBlockNumber  string `json:"current_block_number"`
	HighestBlockHash    string `json:"highest_block_hash"`
	HighestBlockNumber  string `json:"highest_block_number"`
}

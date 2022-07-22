package starknet

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/NethermindEth/juno/internal/services"

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
	Transactions []*felt.Felt `json:"transactions"`
}

type BlockHeader struct {
	BlockHash   *felt.Felt `json:"block_hash"`
	ParentHash  *felt.Felt `json:"parent_hash"`
	BlockNumber uint64     `json:"block_number"`
	NewRoot     *felt.Felt `json:"new_root"`
	Timestamp   int64      `json:"timestamp"`
	Sequencer   *felt.Felt `json:"sequencer_address"`
}

type BlockWithTxHashes struct {
	BlockStatus string `json:"status"`
	BlockHeader
	BlockBodyWithTxHashes
}

func NewBlockWithTxHashes(block *types.Block) *BlockWithTxHashes {
	return &BlockWithTxHashes{
		BlockStatus: block.Status.String(),
		BlockHeader: BlockHeader{
			BlockHash:   block.BlockHash,
			ParentHash:  block.ParentHash,
			BlockNumber: block.BlockNumber,
			NewRoot:     block.NewRoot,
			Timestamp:   block.TimeStamp,
			Sequencer:   block.Sequencer,
		},
		BlockBodyWithTxHashes: BlockBodyWithTxHashes{
			Transactions: block.TxHashes,
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

func NewBlockWithTxs(block *types.Block) (*BlockWithTxs, error) {
	txns := make([]Txn, block.TxCount)
	for i, txHash := range block.TxHashes {
		tx, err := services.TransactionService.GetTransaction(txHash)
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
			BlockHash:   block.BlockHash,
			ParentHash:  block.ParentHash,
			BlockNumber: block.BlockNumber,
			NewRoot:     block.NewRoot,
			Timestamp:   block.TimeStamp,
			Sequencer:   block.Sequencer,
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
	TxnHash   *felt.Felt   `json:"txn_hash"`
	MaxFee    *felt.Felt   `json:"max_fee"`
	Version   string       `json:"version"`
	Signature []*felt.Felt `json:"signature"`
	Nonce     *felt.Felt   `json:"nonce"`
	Type      string       `json:"type"`
}

type FunctionCall struct {
	ContractAddress    *felt.Felt   `json:"contract_address"`
	EntryPointSelector *felt.Felt   `json:"entry_point_selector"`
	Calldata           []*felt.Felt `json:"calldata"`
}

type InvokeTxn struct {
	CommonTxnProperties
	FunctionCall
}

func NewInvokeTxn(txn *types.TransactionInvoke) *InvokeTxn {
	return &InvokeTxn{
		CommonTxnProperties: CommonTxnProperties{
			TxnHash:   txn.Hash,
			MaxFee:    txn.MaxFee,
			Version:   "0x0", // XXX: hardcoded version for now
			Signature: txn.Signature,
			Nonce:     nil, // TODO: Manage transaction nonce
			Type:      "INVOKE",
		},
		FunctionCall: FunctionCall{
			ContractAddress:    txn.ContractAddress,
			EntryPointSelector: txn.EntryPointSelector,
			Calldata:           txn.CallData,
		},
	}
}

func (*InvokeTxn) isTxn() {}

type DeclareTxn struct {
	CommonTxnProperties
	ClassHash     *felt.Felt `json:"class_hash"`
	SenderAddress *felt.Felt `json:"sender_address"`
}

func NewDeclareTxn(txn *types.TransactionDeclare) *DeclareTxn {
	return &DeclareTxn{
		CommonTxnProperties: CommonTxnProperties{
			TxnHash:   txn.Hash,
			MaxFee:    txn.MaxFee,
			Version:   "0x0", // XXX: hardcoded version for now
			Signature: txn.Signature,
			Nonce:     nil, // TODO: Manage transaction nonce
			Type:      "DECLARE",
		},
		ClassHash:     txn.ClassHash,
		SenderAddress: txn.SenderAddress,
	}
}

func (*DeclareTxn) isTxn() {}

type DeployTxn struct {
	TxnHash             *felt.Felt   `json:"txn_hash"`
	ClassHash           *felt.Felt   `json:"class_hash"`
	ContractAddress     *felt.Felt   `json:"contract_address"`
	ConstructorCalldata []*felt.Felt `json:"constructor_calldata"`
}

func NewDeployTxn(txn *types.TransactionDeploy) *DeployTxn {
	return &DeployTxn{
		TxnHash:             txn.Hash,
		ClassHash:           nil, // TODO: manage the class hash
		ContractAddress:     txn.ContractAddress,
		ConstructorCalldata: txn.ConstructorCallData,
	}
}

func (*DeployTxn) isTxn() {}

type Receipt interface {
	isReceipt()
}

func NewRecipt(receipt types.TxnReceipt) (Receipt, error) {
	// TODO: manage the declare txn type
	return nil, errors.New("invalid transaction type")
}

type CommonReceiptProperties struct {
	TxnHash     *felt.Felt `json:"txn_hash"`
	ActualFee   *felt.Felt `json:"actual_fee"`
	Status      string     `json:"status"`
	StatusData  string     `json:"status_data"`
	BlockHash   *felt.Felt `json:"block_hash"`
	BlockNumber uint64     `json:"block_number"`
}

type InvokeTxReceipt struct {
	CommonTxnProperties
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
	ToAddress *felt.Felt   `json:"to_address"`
	Payload   []*felt.Felt `json:"payload"`
}

type MsgToL2 struct {
	// XXX: Use go-ethereum type?
	FromAddress string       `json:"from_address"`
	Payload     []*felt.Felt `json:"payload"`
}

type Event struct {
	FromAddress *felt.Felt   `json:"from_address"`
	Keys        []*felt.Felt `json:"keys"`
	Data        []*felt.Felt `json:"data"`
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
	Offset   string     `json:"offset"`
	Selector *felt.Felt `json:"selector"`
}

type FeeEstimate struct {
	GasConsumed string `json:"gas_consumed"`
	GasPrice    string `json:"gas_price"`
	OverallFee  string `json:"overall_fee"`
}

type SyncStatus struct {
	StartingBlockHash   *felt.Felt `json:"starting_block_hash"`
	StartingBlockNumber *felt.Felt `json:"starting_block_number"`
	CurrentBlockHash    *felt.Felt `json:"current_block_hash"`
	CurrentBlockNumber  *felt.Felt `json:"current_block_number"`
	HighestBlockHash    *felt.Felt `json:"highest_block_hash"`
	HighestBlockNumber  *felt.Felt `json:"highest_block_number"`
}

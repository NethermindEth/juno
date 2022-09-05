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
	blockIdUnknown BlockIdType = iota
	blockIdHash
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
	n, ok := id.value.(uint64)
	return n, ok
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
		if err != nil || value < 0 {
			return ErrInvalidBlockId
		}
		id.idType = blockIdNumber
		id.value = uint64(value)
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
	TxnHash   string   `json:"txn_hash"`
	MaxFee    string   `json:"max_fee"`
	Version   string   `json:"version"`
	Signature []string `json:"signature"`
	Nonce     string   `json:"nonce"`
	Type      string   `json:"type"`
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
	Version             string   `json:"version"`
	Type                string   `json:"type"`
	ContractAddress     string   `json:"contract_address"`
	ContractAddressSalt string   `json:"contract_address_salt"`
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
		Version:             "0x0", // XXX: hardcoded version for now
		Type:                "DEPLOY",
		ContractAddress:     txn.ContractAddress.Hex0x(),
		ContractAddressSalt: txn.ContractAddressSalt.Hex0x(),
		ConstructorCalldata: callData,
	}
}

func (*DeployTxn) isTxn() {}

type TxnType string

const (
	TxnDeclare   TxnType = "DECLARE"
	TxnDeploy    TxnType = "DEPLOY"
	TxnInvoke    TxnType = "INVOKE"
	TxnL1Handler TxnType = "L1_HANDLER"
)

type Receipt interface {
	isReceipt()
}

func NewReceipt(receipt types.TxnReceipt) (Receipt, error) {
	switch receipt := receipt.(type) {
	case *types.TxnInvokeReceipt:
		messagesSent := make([]*MsgToL1, 0, len(receipt.MessagesSent))
		for _, msg := range receipt.MessagesSent {
			messagesSent = append(messagesSent, NewMsgToL1(msg))
		}
		events := make([]*Event, len(receipt.Events))
		for i, e := range receipt.Events {
			events[i] = NewEvent(e)
		}
		return &InvokeTxReceipt{
			CommonReceiptProperties: CommonReceiptProperties{
				TransactionHash: receipt.TxnHash.Hex0x(),
				ActualFee:       receipt.ActualFee.Hex0x(),
				Status:          receipt.Status.String(),
				BlockHash:       receipt.BlockHash.Hex0x(),
				BlockNumber:     receipt.BlockNumber,
				Type:            TxnInvoke,
			},
			MessagesSent:    messagesSent,
			L1OriginMessage: NewMsgToL2(receipt.L1OriginMessage),
			Events:          events,
		}, nil
	case *types.TxnDeployReceipt:
		return &DeployTxReceipt{
			CommonReceiptProperties: CommonReceiptProperties{
				TransactionHash: receipt.TxnHash.Hex0x(),
				ActualFee:       receipt.ActualFee.Hex0x(),
				Status:          receipt.Status.String(),
				BlockHash:       receipt.BlockHash.Hex0x(),
				BlockNumber:     receipt.BlockNumber,
				Type:            TxnDeploy,
			},
		}, nil
	case *types.TxnDeclareReceipt:
		return &DeclareTxReceipt{
			CommonReceiptProperties: CommonReceiptProperties{
				TransactionHash: receipt.TxnHash.Hex0x(),
				ActualFee:       receipt.ActualFee.Hex0x(),
				Status:          receipt.Status.String(),
				BlockHash:       receipt.BlockHash.Hex0x(),
				BlockNumber:     receipt.BlockNumber,
				Type:            TxnDeclare,
			},
		}, nil
	default:
		return nil, errors.New("invalid receipt type")
	}
}

type CommonReceiptProperties struct {
	TransactionHash string  `json:"transaction_hash"`
	ActualFee       string  `json:"actual_fee"`
	Status          string  `json:"status"`
	BlockHash       string  `json:"block_hash"`
	BlockNumber     uint64  `json:"block_number"`
	Type            TxnType `json:"type"`
}

type InvokeTxReceipt struct {
	CommonReceiptProperties
	MessagesSent    []*MsgToL1 `json:"messages_sent"`
	L1OriginMessage *MsgToL2   `json:"l1_origin_message,omitempty"`
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
		return nil
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

type StorageKey string

func (s *StorageKey) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if !isStorageKey(str) {
		return errors.New("invalid storage key")
	}
	*s = StorageKey(str)
	return nil
}

func (s *StorageKey) Felt() *felt.Felt {
	return new(felt.Felt).SetHex(string(*s))
}

type RpcFelt string

func (r *RpcFelt) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if !isFelt(str) {
		return errors.New("invalid felt")
	}
	*r = RpcFelt(str)
	return nil
}

func (r *RpcFelt) Felt() *felt.Felt {
	return new(felt.Felt).SetHex(string(*r))
}

type StorageDiffItem struct {
	Address string `json:"address"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

type DeployedContractItem struct {
	Address   string `json:"address"`
	ClassHash string `json:"class_hash"`
}

type DeclatedContractItem struct {
	ClassHash string `json:"class_hash"`
}

type StateDiff struct {
	StorageDiffs      []*StorageDiffItem      `json:"storage_diffs"`
	DeployedContracts []*DeployedContractItem `json:"deployed_contracts"`
	DeclaredContracts []*DeclatedContractItem `json:"declared_contracts"`
}

type StateUpdate struct {
	BlockHash string     `json:"block_hash"`
	NewRoot   string     `json:"new_root"`
	OldRoot   string     `json:"old_root"`
	StateDiff *StateDiff `json:"state_diff"`
}

func NewStateUpdate(s *types.StateUpdate) *StateUpdate {
	stateDiff := &StateDiff{
		StorageDiffs:      make([]*StorageDiffItem, 0, len(s.StorageDiff)),
		DeployedContracts: make([]*DeployedContractItem, len(s.DeployedContracts)),
		DeclaredContracts: make([]*DeclatedContractItem, len(s.DeclaredContracts)),
	}
	for address, diffs := range s.StorageDiff {
		for _, diff := range diffs {
			stateDiff.StorageDiffs = append(stateDiff.StorageDiffs, &StorageDiffItem{
				Address: address.Hex0x(),
				Key:     diff.Address.Hex0x(),
				Value:   diff.Value.Hex0x(),
			})
		}
	}
	for i, deployedContract := range s.DeployedContracts {
		stateDiff.DeployedContracts[i] = &DeployedContractItem{
			Address:   deployedContract.Address.Hex0x(),
			ClassHash: deployedContract.Hash.Hex0x(),
		}
	}
	for i, declaredContract := range s.DeclaredContracts {
		stateDiff.DeclaredContracts[i] = &DeclatedContractItem{
			ClassHash: declaredContract.Hex0x(),
		}
	}
	return &StateUpdate{
		BlockHash: s.BlockHash.Hex0x(),
		NewRoot:   s.NewRoot.Hex0x(),
		OldRoot:   s.OldRoot.Hex0x(),
		StateDiff: stateDiff,
	}
}

type Status struct {
	Available bool `json:"available"`
}

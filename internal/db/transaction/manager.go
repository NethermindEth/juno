package transaction

import (
	"fmt"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/protobuf/proto"

	"github.com/NethermindEth/juno/pkg/felt"
)

// Manager manages all the related to the database of Transactions. All the
// communications with the transactions' database must be made with this manager.
// Transactions can have two types: DeployTransaction and InvokeFunctionTransaction.
type Manager struct {
	txDb      db.Database
	receiptDb db.Database
}

// NewManager returns a new instance of the Manager.
func NewManager(txDb, receiptDb db.Database) *Manager {
	return &Manager{txDb, receiptDb}
}

// PutTransaction stores new transactions in the database. This method does not
// check if the key already exists. In the case, that the key already exists the
// value is overwritten.
func (m *Manager) PutTransaction(txHash *felt.Felt, tx types.IsTransaction) {
	rawData, err := marshalTransaction(tx)
	if err != nil {
		// notest
		log.Default.With("error", err).Panic("error marshalling Transaction")
	}
	err = m.txDb.Put(txHash.ByteSlice(), rawData)
	if err != nil {
		// notest
		log.Default.With("error", err).Panicf("database error")
	}
}

// GetTransaction searches in the database for the transaction associated with the
// given key. If the key does not exist then returns nil.
func (m *Manager) GetTransaction(txHash *felt.Felt) types.IsTransaction {
	rawData, err := m.txDb.Get(txHash.ByteSlice())
	if err != nil {
		// notest
		log.Default.With("error", err).Panicf("database error")
	}
	// Check not found
	if rawData == nil {
		// notest
		return nil
	}
	tx, err := unmarshalTransaction(rawData)
	if err != nil {
		// notest
		log.Default.With("error", err).Panicf("unmarshalling error")
	}
	return tx
}

// PutReceipt stores  new transactions receipts in the database. This method
// does not check if the key already exists. In the case, that the key already
// exists the value is overwritten.
func (m *Manager) PutReceipt(txHash *felt.Felt, txReceipt *types.TransactionReceipt) {
	rawData, err := marshalTransactionReceipt(txReceipt)
	if err != nil {
		// notest
		log.Default.With("error", err).Panic("error marshaling TransactionReceipt")
	}
	err = m.receiptDb.Put(txHash.ByteSlice(), rawData)
	if err != nil {
		// notest
		log.Default.With("error", err).Panic("database error")
	}
}

// GetReceipt searches in the database for the transaction receipt associated
// with the given key. If the key does not exist then returns nil.
func (m *Manager) GetReceipt(txHash *felt.Felt) *types.TransactionReceipt {
	rawData, err := m.receiptDb.Get(txHash.ByteSlice())
	if err != nil {
		// notest
		log.Default.With("error", err).Panicf("database error")
	}
	// Check not found
	if rawData == nil {
		// notest
		return nil
	}
	receipt, err := unmarshalTransactionReceipt(rawData)
	if err != nil {
		// notest
		log.Default.With("error", err).Panicf("unmarshalling error")
	}
	return receipt
}

// Close closes the manager, specific the associated database.
func (m *Manager) Close() {
	m.txDb.Close()
	m.receiptDb.Close()
}

func marshalTransaction(transaction types.IsTransaction) ([]byte, error) {
	protoTx := Transaction{
		Hash: transaction.GetHash().ByteSlice(),
	}
	switch tx := transaction.(type) {
	case *types.TransactionDeploy:
		deploy := Deploy{
			ContractAddressSalt: tx.ContractAddress.ByteSlice(),
			ConstructorCallData: marshalFelts(tx.ConstructorCallData),
		}
		protoTx.Tx = &Transaction_Deploy{&deploy}
	case *types.TransactionInvoke:
		invoke := InvokeFunction{
			ContractAddress:    tx.ContractAddress.ByteSlice(),
			EntryPointSelector: tx.EntryPointSelector.ByteSlice(),
			CallData:           marshalFelts(tx.CallData),
			Signature:          marshalFelts(tx.Signature),
			MaxFee:             tx.MaxFee.ByteSlice(),
		}
		protoTx.Tx = &Transaction_Invoke{&invoke}
	}
	return proto.Marshal(&protoTx)
}

func unmarshalTransaction(b []byte) (types.IsTransaction, error) {
	var protoTx Transaction
	err := proto.Unmarshal(b, &protoTx)
	if err != nil {
		return nil, err
	}
	if tx := protoTx.GetInvoke(); tx != nil {
		out := types.TransactionInvoke{
			Hash:               new(felt.Felt).SetBytes(protoTx.Hash),
			ContractAddress:    new(felt.Felt).SetBytes(tx.ContractAddress),
			EntryPointSelector: new(felt.Felt).SetBytes(tx.EntryPointSelector),
			CallData:           unmarshalFelts(tx.CallData),
			Signature:          unmarshalFelts(tx.Signature),
			MaxFee:             new(felt.Felt).SetBytes(tx.MaxFee),
		}
		return &out, nil
	}
	if tx := protoTx.GetDeploy(); tx != nil {
		out := types.TransactionDeploy{
			Hash:                new(felt.Felt).SetBytes(protoTx.Hash),
			ContractAddress:     new(felt.Felt).SetBytes(tx.ContractAddressSalt),
			ConstructorCallData: unmarshalFelts(tx.ConstructorCallData),
		}
		return &out, nil
	}
	// notest
	return nil, fmt.Errorf("unexpected transaction type")
}

func marshalTransactionReceipt(receipt *types.TransactionReceipt) ([]byte, error) {
	protoReceipt := TransactionReceipt{
		TxHash:          receipt.TxHash.ByteSlice(),
		ActualFee:       receipt.ActualFee.ByteSlice(),
		Status:          marshalTransactionStatus(receipt.Status),
		StatusData:      receipt.StatusData,
		L1OriginMessage: marshalMessageL1ToL2(receipt.L1OriginMessage),
	}
	if receipt.MessagesSent != nil {
		protoReceipt.MessagesSent = make([]*MessageToL1, len(receipt.MessagesSent))
		for i, message := range receipt.MessagesSent {
			protoReceipt.MessagesSent[i] = marshalMessageL2ToL1(&message)
		}
	}
	if receipt.Events != nil {
		protoReceipt.Events = make([]*Event, len(receipt.Events))
		for i, event := range receipt.Events {
			protoReceipt.Events[i] = marshalEvent(&event)
		}
	}
	return proto.Marshal(&protoReceipt)
}

func unmarshalTransactionReceipt(b []byte) (*types.TransactionReceipt, error) {
	var protoReceipt TransactionReceipt
	err := proto.Unmarshal(b, &protoReceipt)
	if err != nil {
		return nil, err
	}
	receipt := &types.TransactionReceipt{
		TxHash:          new(felt.Felt).SetBytes(protoReceipt.TxHash),
		ActualFee:       new(felt.Felt).SetBytes(protoReceipt.ActualFee),
		Status:          unmarshalTransactionStatus(protoReceipt.Status),
		StatusData:      protoReceipt.StatusData,
		L1OriginMessage: unmarshalMessageL1ToL2(protoReceipt.L1OriginMessage),
		Events:          nil,
	}
	if protoReceipt.MessagesSent != nil {
		receipt.MessagesSent = make([]types.MessageL2ToL1, len(protoReceipt.MessagesSent))
		for i, message := range protoReceipt.MessagesSent {
			receipt.MessagesSent[i] = unmarshalMessageL2ToL1(message)
		}
	}
	if protoReceipt.Events != nil {
		receipt.Events = make([]types.Event, len(protoReceipt.Events))
		for i, event := range protoReceipt.Events {
			receipt.Events[i] = unmarshalEvent(event)
		}
	}
	return receipt, nil
}

func marshalTransactionStatus(status types.TransactionStatus) Status {
	// notest
	switch status {
	case types.TxStatusUnknown:
		return Status_UNKNOWN
	case types.TxStatusReceived:
		return Status_RECEIVED
	case types.TxStatusPending:
		return Status_PENDING
	case types.TxStatusAcceptedOnL2:
		return Status_ACCEPTED_ON_L2
	case types.TxStatusAcceptedOnL1:
		return Status_ACCEPTED_ON_L1
	case types.TxStatusRejected:
		return Status_REJECTED
	default:
		return Status_UNKNOWN
	}
}

func unmarshalTransactionStatus(status Status) types.TransactionStatus {
	// notest
	switch status {
	case Status_UNKNOWN:
		return types.TxStatusUnknown
	case Status_RECEIVED:
		return types.TxStatusReceived
	case Status_PENDING:
		return types.TxStatusPending
	case Status_ACCEPTED_ON_L2:
		return types.TxStatusAcceptedOnL2
	case Status_ACCEPTED_ON_L1:
		return types.TxStatusAcceptedOnL1
	case Status_REJECTED:
		return types.TxStatusRejected
	default:
		return types.TxStatusUnknown
	}
}

func marshalMessageL2ToL1(message *types.MessageL2ToL1) *MessageToL1 {
	return &MessageToL1{
		ToAddress: message.ToAddress.Bytes(),
		Payload:   marshalFelts(message.Payload),
	}
}

func unmarshalMessageL2ToL1(message *MessageToL1) types.MessageL2ToL1 {
	return types.MessageL2ToL1{
		ToAddress: types.BytesToEthAddress(message.ToAddress),
		Payload:   unmarshalFelts(message.Payload),
	}
}

func marshalMessageL1ToL2(message *types.MessageL1ToL2) *MessageToL2 {
	if message == nil {
		return nil
	}
	return &MessageToL2{
		FromAddress: message.FromAddress.Bytes(),
		Payload:     marshalFelts(message.Payload),
	}
}

func unmarshalMessageL1ToL2(message *MessageToL2) *types.MessageL1ToL2 {
	if message == nil {
		// notest
		return nil
	}
	return &types.MessageL1ToL2{
		FromAddress: types.BytesToEthAddress(message.FromAddress),
		Payload:     unmarshalFelts(message.Payload),
	}
}

func marshalEvent(event *types.Event) *Event {
	return &Event{
		FromAddress: event.FromAddress.ByteSlice(),
		Keys:        marshalFelts(event.Keys),
		Data:        marshalFelts(event.Data),
	}
}

func unmarshalEvent(event *Event) types.Event {
	return types.Event{
		FromAddress: new(felt.Felt).SetBytes(event.FromAddress),
		Keys:        unmarshalFelts(event.Keys),
		Data:        unmarshalFelts(event.Data),
	}
}

func marshalFelts(felts []*felt.Felt) [][]byte {
	out := make([][]byte, len(felts))
	for i, felt := range felts {
		out[i] = felt.ByteSlice()
	}
	return out
}

func unmarshalFelts(bs [][]byte) []*felt.Felt {
	if bs == nil {
		return nil
	}
	out := make([]*felt.Felt, len(bs))
	for i, b := range bs {
		out[i] = new(felt.Felt).SetBytes(b)
	}
	return out
}

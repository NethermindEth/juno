package transaction

import (
	"fmt"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"
	"google.golang.org/protobuf/proto"
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
func (m *Manager) PutTransaction(txHash *felt.Felt, tx types.IsTransaction) error {
	rawData, err := marshalTransaction(tx)
	if err != nil {
		return err
	}
	if err := m.txDb.Put(txHash.ByteSlice(), rawData); err != nil {
		return err
	}
	return nil
}

// GetTransaction searches in the database for the transaction associated with the
// given key. If the key does not exist then returns nil.
func (m *Manager) GetTransaction(txHash *felt.Felt) (types.IsTransaction, error) {
	rawData, err := m.txDb.Get(txHash.ByteSlice())
	if err != nil {
		return nil, err
	}
	tx, err := unmarshalTransaction(rawData)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// PutReceipt stores  new transactions receipts in the database. This method
// does not check if the key already exists. In the case, that the key already
// exists the value is overwritten.
func (m *Manager) PutReceipt(txHash *felt.Felt, txReceipt types.TxnReceipt) error {
	rawData, err := marshalTransactionReceipt(txReceipt)
	if err != nil {
		return err
	}
	if err := m.receiptDb.Put(txHash.ByteSlice(), rawData); err != nil {
		return err
	}
	return nil
}

// GetReceipt searches in the database for the transaction receipt associated
// with the given key. If the key does not exist then returns nil.
func (m *Manager) GetReceipt(txHash *felt.Felt) (types.TxnReceipt, error) {
	rawData, err := m.receiptDb.Get(txHash.ByteSlice())
	if err != nil {
		return nil, err
	}
	receipt, err := unmarshalTransactionReceipt(rawData)
	if err != nil {
		return nil, err
	}
	return receipt, nil
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
			ClassHash:           tx.ClassHash.ByteSlice(),
			ContractAddress:     tx.ContractAddress.ByteSlice(),
			ContractAddressSalt: tx.ContractAddressSalt.ByteSlice(),
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
			Version:            tx.Version.ByteSlice(),
		}
		protoTx.Tx = &Transaction_Invoke{&invoke}
	case *types.TransactionDeclare:
		declare := Declare{
			ClassHash:     tx.ClassHash.ByteSlice(),
			SenderAddress: tx.SenderAddress.ByteSlice(),
			MaxFee:        tx.MaxFee.ByteSlice(),
			Signature:     marshalFelts(tx.Signature),
			Nonce:         tx.Nonce.ByteSlice(),
		}
		protoTx.Tx = &Transaction_Declare{&declare}
	}
	return proto.Marshal(&protoTx)
}

func unmarshalTransaction(b []byte) (types.IsTransaction, error) {
	var protoTx Transaction
	if err := proto.Unmarshal(b, &protoTx); err != nil {
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
			Version:            new(felt.Felt).SetBytes(tx.Version),
		}
		return &out, nil
	}
	if tx := protoTx.GetDeploy(); tx != nil {
		out := types.TransactionDeploy{
			Hash:                new(felt.Felt).SetBytes(protoTx.Hash),
			ClassHash:           new(felt.Felt).SetBytes(tx.ClassHash),
			ContractAddress:     new(felt.Felt).SetBytes(tx.ContractAddress),
			ContractAddressSalt: new(felt.Felt).SetBytes(tx.ContractAddressSalt),
			ConstructorCallData: unmarshalFelts(tx.ConstructorCallData),
		}
		return &out, nil
	}
	if tx := protoTx.GetDeclare(); tx != nil {
		out := types.TransactionDeclare{
			Hash:          new(felt.Felt).SetBytes(protoTx.Hash),
			ClassHash:     new(felt.Felt).SetBytes(tx.ClassHash),
			SenderAddress: new(felt.Felt).SetBytes(tx.SenderAddress),
			MaxFee:        new(felt.Felt).SetBytes(tx.MaxFee),
			Signature:     unmarshalFelts(tx.Signature),
			Nonce:         new(felt.Felt).SetBytes(tx.Nonce),
			Version:       new(felt.Felt).SetBytes(tx.Version),
		}
		return &out, nil
	}
	// notest
	return nil, fmt.Errorf("unexpected transaction type")
}

func marshalTransactionReceipt(receipt types.TxnReceipt) ([]byte, error) {
	var protoReceipt Receipt
	switch r := receipt.(type) {
	case *types.TxnInvokeReceipt:
		messagesSent := make([]*MsgToL1, len(r.MessagesSent))
		for i, msg := range r.MessagesSent {
			messagesSent[i] = marshalMessageL2ToL1(msg)
		}
		events := make([]*Event, len(r.Events))
		for i, ev := range r.Events {
			events[i] = marshalEvent(ev)
		}
		protoReceipt.Receipt = &Receipt_Invoke{
			Invoke: &InvokeReceipt{
				Common: &ReceiptCommon{
					TxnHash:     r.TxnHash.ByteSlice(),
					ActualFee:   r.ActualFee.ByteSlice(),
					Status:      marshalTransactionStatus(r.Status),
					StatusData:  r.StatusData,
					BlockHash:   r.BlockHash.ByteSlice(),
					BlockNumber: r.BlockNumber,
				},
				MessagesSent:    messagesSent,
				L1OriginMessage: marshalMessageL1ToL2(r.L1OriginMessage),
				Events:          events,
			},
		}
	case *types.TxnDeclareReceipt:
		protoReceipt.Receipt = &Receipt_Declare{
			Declare: &DeclareReceipt{
				Common: &ReceiptCommon{
					TxnHash:     r.TxnHash.ByteSlice(),
					ActualFee:   r.ActualFee.ByteSlice(),
					Status:      marshalTransactionStatus(r.Status),
					StatusData:  r.StatusData,
					BlockHash:   r.BlockHash.ByteSlice(),
					BlockNumber: r.BlockNumber,
				},
			},
		}
	case *types.TxnDeployReceipt:
		protoReceipt.Receipt = &Receipt_Deploy{
			Deploy: &DeployReceipt{
				Common: &ReceiptCommon{
					TxnHash:     r.TxnHash.ByteSlice(),
					ActualFee:   r.ActualFee.ByteSlice(),
					Status:      marshalTransactionStatus(r.Status),
					StatusData:  r.StatusData,
					BlockHash:   r.BlockHash.ByteSlice(),
					BlockNumber: r.BlockNumber,
				},
			},
		}
	default:
		panic("unexpected type")
	}
	return proto.Marshal(&protoReceipt)
}

func unmarshalTransactionReceipt(b []byte) (types.TxnReceipt, error) {
	var protoReceipt Receipt
	if err := proto.Unmarshal(b, &protoReceipt); err != nil {
		return nil, err
	}
	if receipt := protoReceipt.GetInvoke(); receipt != nil {
		var messagesSent []*types.MsgToL1 = nil
		if len(receipt.MessagesSent) > 0 {
			messagesSent = make([]*types.MsgToL1, len(receipt.MessagesSent))
			for i, msg := range receipt.MessagesSent {
				messagesSent[i] = unmarshalMessageL2ToL1(msg)
			}
		}
		var events []*types.Event = nil
		if len(receipt.Events) > 0 {
			events = make([]*types.Event, len(receipt.Events))
			for i, event := range receipt.Events {
				events[i] = unmarshalEvent(event)
			}
		}
		return &types.TxnInvokeReceipt{
			TxnReceiptCommon: types.TxnReceiptCommon{
				TxnHash:     new(felt.Felt).SetBytes(receipt.Common.TxnHash),
				ActualFee:   new(felt.Felt).SetBytes(receipt.Common.ActualFee),
				Status:      unmarshalTransactionStatus(receipt.Common.Status),
				StatusData:  receipt.Common.StatusData,
				BlockHash:   new(felt.Felt).SetBytes(receipt.Common.BlockHash),
				BlockNumber: receipt.Common.BlockNumber,
			},
			MessagesSent:    messagesSent,
			L1OriginMessage: unmarshalMessageL1ToL2(receipt.L1OriginMessage),
			Events:          events,
		}, nil
	}
	if receipt := protoReceipt.GetDeclare(); receipt != nil {
		return &types.TxnDeclareReceipt{
			TxnReceiptCommon: types.TxnReceiptCommon{
				TxnHash:     new(felt.Felt).SetBytes(receipt.Common.TxnHash),
				ActualFee:   new(felt.Felt).SetBytes(receipt.Common.ActualFee),
				Status:      unmarshalTransactionStatus(receipt.Common.Status),
				StatusData:  receipt.Common.StatusData,
				BlockHash:   new(felt.Felt).SetBytes(receipt.Common.BlockHash),
				BlockNumber: receipt.Common.BlockNumber,
			},
		}, nil
	}
	if receipt := protoReceipt.GetDeploy(); receipt != nil {
		return &types.TxnDeployReceipt{
			TxnReceiptCommon: types.TxnReceiptCommon{
				TxnHash:     new(felt.Felt).SetBytes(receipt.Common.TxnHash),
				ActualFee:   new(felt.Felt).SetBytes(receipt.Common.ActualFee),
				Status:      unmarshalTransactionStatus(receipt.Common.Status),
				StatusData:  receipt.Common.StatusData,
				BlockHash:   new(felt.Felt).SetBytes(receipt.Common.BlockHash),
				BlockNumber: receipt.Common.BlockNumber,
			},
		}, nil
	}
	panic("unexpected type")
}

func marshalTransactionStatus(status types.TxnStatus) Status {
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

func unmarshalTransactionStatus(status Status) types.TxnStatus {
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

func marshalMessageL2ToL1(message *types.MsgToL1) *MsgToL1 {
	return &MsgToL1{
		FromAddress: message.FromAddress.ByteSlice(),
		ToAddress:   message.ToAddress.Bytes(),
		Payload:     marshalFelts(message.Payload),
	}
}

func unmarshalMessageL2ToL1(message *MsgToL1) *types.MsgToL1 {
	return &types.MsgToL1{
		FromAddress: new(felt.Felt).SetBytes(message.FromAddress),
		ToAddress:   types.BytesToEthAddress(message.ToAddress),
		Payload:     unmarshalFelts(message.Payload),
	}
}

func marshalMessageL1ToL2(message *types.MsgToL2) *MsgToL2 {
	if message == nil {
		// notest
		return nil
	}
	return &MsgToL2{
		FromAddress: message.FromAddress.Bytes(),
		ToAddress:   message.ToAddress.ByteSlice(),
		Payload:     marshalFelts(message.Payload),
	}
}

func unmarshalMessageL1ToL2(message *MsgToL2) *types.MsgToL2 {
	if message == nil {
		// notest
		return nil
	}
	return &types.MsgToL2{
		FromAddress: types.BytesToEthAddress(message.FromAddress),
		ToAddress:   new(felt.Felt).SetBytes(message.ToAddress),
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

func unmarshalEvent(event *Event) *types.Event {
	return &types.Event{
		FromAddress: new(felt.Felt).SetBytes(event.FromAddress),
		Keys:        unmarshalFelts(event.Keys),
		Data:        unmarshalFelts(event.Data),
	}
}

func marshalFelts(felts []*felt.Felt) [][]byte {
	out := make([][]byte, len(felts))
	for i, f := range felts {
		out[i] = f.ByteSlice()
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

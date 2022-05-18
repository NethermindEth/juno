package transaction

import (
	"encoding/json"
	"fmt"
	"github.com/NethermindEth/juno/internal/log"
	"math/big"
)

// TxType is an enum representing all the possible transaction types.
type TxType int64

const (
	TxDeploy TxType = iota
	TxInvokeFunction
)

// Common has all the common fields between all the transaction types.
type Common struct {
	TxHash big.Int
	TxType TxType
}

// DeployFields has all the fields unique for a Deploy transaction.
type DeployFields struct {
	ContractAddressSalt big.Int
	ConstructorCalldata []big.Int
}

// InvokeFunctionFields has all the fields unique for an Invoke Function
// transaction.
type InvokeFunctionFields struct {
	ContractAddress    big.Int
	EntryPointSelector big.Int
	CallData           []big.Int
	Signature          []big.Int
	MaxFee             big.Int
}

// Transaction has accessible only the common fields described in the Common
// type. But, at the same time has (private) the fields of the specific
// transaction and can be obtained calling the proper method like AsDeploy or
// AsInvokeFunction, if the invoked conversion does not match with the real type
// then the conversion panics.
type Transaction struct {
	Common
	specificFields interface{}
}

// transactionContainer is a container used to store the values in the database.
type transactionContainer struct {
	Common
	SpecificFields json.RawMessage
}

// AsDeploy converts the Transaction into a DeployTransaction. If the
// Transaction is not a DeployTransaction, then panics.
func (tx *Transaction) AsDeploy() *DeployTransaction {
	if tx.TxType != TxDeploy {
		log.Default.Panicf("can't convert transaction of type %d to type %d", tx.TxType, TxDeploy)
	}
	fields, ok := tx.specificFields.(DeployFields)
	if !ok {
		log.Default.Panicf("TxType missmatch with the specificFields type")
	}
	return &DeployTransaction{
		tx.Common,
		fields,
	}
}

// AsInvokeFunction converts the Transaction into an InvokeFunctionTransaction.
// If the Transaction is not an InvokeFunctionTransaction, then panics.
func (tx *Transaction) AsInvokeFunction() *InvokeFunctionTransaction {
	if tx.TxType != TxInvokeFunction {
		log.Default.Panicf("can't convert transaction of type %d to type %d", tx.TxType, TxInvokeFunction)
	}
	fields, ok := tx.specificFields.(InvokeFunctionFields)
	if !ok {
		log.Default.Panicf("TxType missmatch with the specificFields type")
	}
	return &InvokeFunctionTransaction{
		tx.Common,
		fields,
	}
}

// Marshal is used to storing the transaction as an array of bytes in the
// database.
func (tx *Transaction) Marshal() ([]byte, error) {
	container := &transactionContainer{
		Common: tx.Common,
	}
	switch tx.TxType {
	case TxDeploy:
		fields, ok := tx.specificFields.(DeployFields)
		if !ok {
			return nil, fmt.Errorf("TxType is Deploy but not the fields")
		}
		rawFields, err := json.Marshal(&fields)
		if err != nil {
			return nil, err
		}
		container.SpecificFields = rawFields
	case TxInvokeFunction:
		fields, ok := tx.specificFields.(InvokeFunctionFields)
		if !ok {
			return nil, fmt.Errorf("TxType is InvokeFunction but not the fields")
		}
		rawFields, err := json.Marshal(&fields)
		if err != nil {
			return nil, err
		}
		container.SpecificFields = rawFields
	default:
		return nil, fmt.Errorf("unknown TxType")
	}
	return json.Marshal(container)
}

// Unmarshal is used to decode the values (in bytes) in the database into a
// transaction.
func (tx *Transaction) Unmarshal(data []byte) error {
	container := new(transactionContainer)
	err := json.Unmarshal(data, container)
	if err != nil {
		return err
	}
	switch container.TxType {
	case TxDeploy:
		fields := new(DeployFields)
		err = json.Unmarshal(container.SpecificFields, fields)
		if err != nil {
			return err
		}
		tx.specificFields = fields
	case TxInvokeFunction:
		fields := new(InvokeFunctionFields)
		err = json.Unmarshal(container.SpecificFields, fields)
		if err != nil {
			return err
		}
		tx.specificFields = fields
	default:
		return fmt.Errorf("unknown TxType")
	}
	tx.Common = container.Common
	return nil
}

// DeployTransaction represents a deploy transaction. It's a composition of
// Common and DeployFields types.
type DeployTransaction struct {
	Common
	DeployFields
}

// AsTransaction converts a DeployTransaction into a Transaction. After this
// operation, the resulted Transaction can be converted again into a
// DeployTransaction.
func (tx *DeployTransaction) AsTransaction() *Transaction {
	return &Transaction{
		Common:         tx.Common,
		specificFields: tx.DeployFields,
	}
}

// InvokeFunctionTransaction represents an invoke function transaction. It's a
// composition of Common and InvokeFunctionTransaction types.
type InvokeFunctionTransaction struct {
	Common
	InvokeFunctionFields
}

// AsTransaction converts a InvokeFunctionTransaction into a Transaction. After
// this operation, the resulted Transaction can be converted again into an
// InvokeFunctionTransaction.
func (tx *InvokeFunctionTransaction) AsTransaction() *Transaction {
	return &Transaction{
		Common:         tx.Common,
		specificFields: tx.InvokeFunctionFields,
	}
}

package core

import (
	"errors"
	"math/big"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common"
)

type Event struct {
	Data []*felt.Felt
	From *felt.Felt
	Keys []*felt.Felt
}

type L1ToL2Message struct {
	From     common.Address
	Nonce    *felt.Felt
	Payload  []*felt.Felt
	Selector *felt.Felt
	To       *felt.Felt
}

type L2ToL1Message struct {
	From    *felt.Felt
	Payload []*felt.Felt
	To      common.Address
}

type ExecutionResources struct {
	BuiltinInstanceCounter BuiltinInstanceCounter
	MemoryHoles            uint64
	Steps                  uint64
}

type BuiltinInstanceCounter struct {
	Bitwise    uint64
	EcOp       uint64
	Ecsda      uint64
	Output     uint64
	Pedersen   uint64
	RangeCheck uint64
}

// Todo: Having both TransactionType and DeployTransaction,
// InvokeTransaction and DeclareTransaction is redundant
type TransactionType int

const (
	Declare TransactionType = iota
	Deploy
	DeployAccount
	Invoke
	L1Handler
)

type TransactionReceipt struct {
	ActualFee          *felt.Felt
	Events             []*Event
	ExecutionResources *ExecutionResources
	L1ToL2Message      *L1ToL2Message
	L2ToL1Message      []*L2ToL1Message
	Signatures         []*felt.Felt
	TransactionHash    *felt.Felt
	TransactionIndex   *big.Int
	// Todo: Do we need to have this?
	Type TransactionType
}

type Transaction interface {
	// Todo: Add Hash as a field to all the Transaction Objects
	Hash() *felt.Felt
}

type DeployTransaction struct {
	// A random number used to distinguish between different instances of the contract.
	ContractAddressSalt *felt.Felt
	// The address of the contract.
	ContractAddress *felt.Felt
	// The object that defines the contract’s functionality.
	Class Class
	// The arguments passed to the constructor during deployment.
	ConstructorCallData []*felt.Felt
	// Who invoked the deployment. Set to 0 (in future: the deploying account contract).
	CallerAddress *felt.Felt
	// The transaction’s version. Possible values are 1 or 0.
	//
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	// Transaction version 0 is deprecated and will be removed in a future version of StarkNet.
	Version *felt.Felt
}

func (d *DeployTransaction) Hash(chainId []byte) (*felt.Felt, error) {
	snKeccakConstructor, err := crypto.StarkNetKeccak([]byte("constructor"))
	if err != nil {
		return nil, err
	}
	return crypto.PedersenArray(
		new(felt.Felt).SetBytes([]byte("deploy")),
		d.Version,
		d.ContractAddress,
		snKeccakConstructor,
		crypto.PedersenArray(d.ConstructorCallData...),
		new(felt.Felt),
		new(felt.Felt).SetBytes(chainId),
	), nil
}

type InvokeTransaction struct {
	// Version 0 fields
	// The address of the contract invoked by this transaction.
	ContractAddress *felt.Felt
	// The encoding of the selector for the function invoked (the entry point in the contract)
	EntryPointSelector *felt.Felt

	// Version 1 fields
	// The address of the sender of this transaction.
	SenderAddress *felt.Felt
	// The transaction nonce.
	Nonce *felt.Felt
	// The arguments that are passed to the validated and execute functions.
	CallData []*felt.Felt
	// Additional information given by the sender, used to validate the transaction.
	Signature []*felt.Felt
	// The maximum fee that the sender is willing to pay for the transaction
	MaxFee *felt.Felt
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	Version *felt.Felt
}

func (i *InvokeTransaction) Hash(chainId []byte) (*felt.Felt, error) {
	invokeFelt := new(felt.Felt).SetBytes([]byte("invoke"))
	if i.Version.IsZero() {
		return crypto.PedersenArray(
			invokeFelt,
			i.ContractAddress,
			i.EntryPointSelector,
			crypto.PedersenArray(i.CallData...),
			new(felt.Felt).SetBytes(chainId),
		), nil
	} else if i.Version.IsOne() {
		return crypto.PedersenArray(
			invokeFelt,
			i.Version,
			i.SenderAddress,
			new(felt.Felt),
			crypto.PedersenArray(i.CallData...),
			i.MaxFee,
			new(felt.Felt).SetBytes(chainId),
			i.Nonce,
		), nil
	}
	return nil, errors.New("invalid transaction version")
}

type DeclareTransaction struct {
	// The class object.
	Class Class
	// The address of the account initiating the transaction.
	SenderAddress *felt.Felt
	// The maximum fee that the sender is willing to pay for the transaction.
	MaxFee *felt.Felt
	// Additional information given by the sender, used to validate the transaction.
	Signature []*felt.Felt
	// The transaction nonce.
	Nonce *felt.Felt
	// The transaction’s version. Possible values are 1 or 0.
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	// Transaction version 0 is deprecated and will be removed in a future version of StarkNet.
	Version *felt.Felt
}

func (d *DeclareTransaction) Hash(chainId []byte) (*felt.Felt, error) {
	declareFelt := new(felt.Felt).SetBytes([]byte("declare"))
	if d.Version.IsZero() {
		return crypto.PedersenArray(
			declareFelt,
			d.Version,
			d.SenderAddress,
			new(felt.Felt),
			crypto.PedersenArray(make([]*felt.Felt, 0)...),
			d.MaxFee,
			new(felt.Felt).SetBytes(chainId),
			d.Class.Hash(),
		), nil
	} else if d.Version.IsOne() {
		return crypto.PedersenArray(
			declareFelt,
			d.Version,
			d.SenderAddress,
			new(felt.Felt),
			crypto.PedersenArray(d.Class.Hash()),
			d.MaxFee,
			new(felt.Felt).SetBytes(chainId),
			d.Nonce,
		), nil
	}
	return nil, errors.New("invalid transaction version")
}

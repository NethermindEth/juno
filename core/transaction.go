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
	BuiltinInstanceCounter struct {
		Bitwise    uint64
		EcOp       uint64
		Ecsda      uint64
		Output     uint64
		Pedersen   uint64
		RangeCheck uint64
	}
	MemoryHoles uint64
	Steps       uint64
}

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
	L2ToL1Message      *[]L2ToL1Message
	Signatures         []*felt.Felt
	TransactionHash    *felt.Felt
	TransactionIndex   *big.Int
	Type               TransactionType
}

type Transaction interface {
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
	ConstructorCalldata []*felt.Felt
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
	// Implemented pedersen hash as defined here:
	// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_deploy_transaction
	var data []*felt.Felt

	deployFelt := new(felt.Felt).SetBytes([]byte("deploy"))
	data = append(data, deployFelt)

	data = append(data, d.Version)

	// Address of the contract
	data = append(data, d.ContractAddress)

	// sn_keccak("constructor")
	constructorByte := []byte("constructor")
	snKeccakContructor, err := crypto.StarkNetKeccak(constructorByte)
	if err != nil {
		return nil, err
	}
	data = append(data, snKeccakContructor)

	// Pedersen Hash of Constructor Calldata
	pedersenConstructorCalldata := crypto.PedersenArray(d.ConstructorCalldata...)

	data = append(data, pedersenConstructorCalldata)

	zeroFelt := new(felt.Felt).SetBytes([]byte{0})
	data = append(data, zeroFelt)

	chainIdFelt := new(felt.Felt).SetBytes(chainId)
	data = append(data, chainIdFelt)

	deployTransactionHash := crypto.PedersenArray(data...)
	return deployTransactionHash, nil
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

	// The arguments that are passed to the validate and execute functions.
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
	var data []*felt.Felt

	invokeFelt := new(felt.Felt).SetBytes([]byte("invoke"))
	data = append(data, invokeFelt)

	data = append(data, i.Version)
	if i.Version.IsZero() {
		// Implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v1_invoke_transaction
		// Address of the contract
		contractAddress := i.ContractAddress
		data = append(data, contractAddress)

		// EntryPointSelector
		entryPointSelector := i.EntryPointSelector
		data = append(data, entryPointSelector)

		// Pedersen Hash of the Calldata
		pedersenHashCalldata := crypto.PedersenArray(i.CallData...)

		data = append(data, pedersenHashCalldata)

		data = append(data, i.MaxFee)

		chainIdFelt := new(felt.Felt).SetBytes(chainId)
		data = append(data, chainIdFelt)

		invokeTransactionHash := crypto.PedersenArray(data...)

		return invokeTransactionHash, nil
	} else if i.Version.IsOne() {
		// Implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v0_invoke_transaction
		// Transaction sender address
		senderAddress := i.SenderAddress
		data = append(data, senderAddress)

		// Zero Felt
		zeroFelt := new(felt.Felt).SetBytes([]byte{0})
		data = append(data, zeroFelt)

		// Pedersen Hash of the Calldata
		pedersenHashCalldata := crypto.PedersenArray(i.CallData...)
		data = append(data, pedersenHashCalldata)

		data = append(data, i.MaxFee)

		chainIdFelt := new(felt.Felt).SetBytes(chainId)
		data = append(data, chainIdFelt)

		data = append(data, i.Nonce)

		invokeTransactionHash := crypto.PedersenArray(data...)

		return invokeTransactionHash, nil
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
	var data []*felt.Felt

	// Declare Felt
	declareFelt := new(felt.Felt).SetBytes([]byte("declare"))
	data = append(data, declareFelt)

	// Version Felt
	data = append(data, d.Version)

	// Sender Address
	senderAddress := d.SenderAddress
	data = append(data, senderAddress)

	// Zero Felt
	zeroFelt := new(felt.Felt).SetBytes([]byte{0})
	data = append(data, zeroFelt)
	if d.Version.IsZero() {
		// Implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v0_declare_transaction
		// Zero Felt
		data = append(data, zeroFelt)

		// Max Fee
		data = append(data, d.MaxFee)

		// Chain Id
		chainIdFelt := new(felt.Felt).SetBytes(chainId)
		data = append(data, chainIdFelt)

		// Class Hash
		classHash := d.Class.Hash()
		data = append(data, classHash)

		declareTransactionHash := crypto.PedersenArray(data...)

		return declareTransactionHash, nil
	} else if d.Version.IsOne() {
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v1_declare_transaction

		// Class Hash
		classHash := d.Class.Hash()

		// Calculate pedersen hash on class hash elements
		classHash = crypto.PedersenArray(classHash)
		data = append(data, classHash)

		//Max Fee
		data = append(data, d.MaxFee)

		// Chain Id
		chainIdFelt := new(felt.Felt).SetBytes(chainId)
		data = append(data, chainIdFelt)

		// Nonce
		data = append(data, d.Nonce)

		declareTransactionHash := crypto.PedersenArray(data...)

		return declareTransactionHash, nil
	}
	return nil, errors.New("invalid transaction version")
}

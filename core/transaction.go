package core

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/contract"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

type Transaction interface {
	Hash() *felt.Felt
}

type DeployTransaction struct {
	// A random number used to distinguish between different instances of the contract.
	ContractAddressSalt *felt.Felt
	// The object that defines the contract’s functionality.
	Class contract.Class
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
	// Todo: implement pedersen hash as defined here:
	// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_deploy_transaction
	var data []*felt.Felt

	deployFelt := new(felt.Felt).SetBytes([]byte("deploy"))
	data = append(data, deployFelt)

	data = append(data, d.Version)

	// Address of the contract
	zeroFelt := new(felt.Felt).SetZero()
	contractAddress, err := d.Class.Address(zeroFelt, d.ContractAddressSalt, d.ConstructorCalldata)
	fmt.Println("address: ", contractAddress)
	if err != nil {
		return nil, err
	}
	data = append(data, contractAddress)

	// sn_keccak("constructor")
	constructorByte := []byte("constructor")
	snKeccakContructor, err := crypto.StarkNetKeccak(constructorByte)
	if err != nil {
		return nil, err
	}
	data = append(data, snKeccakContructor)

	// Pedersen Hash of Constructor Calldata
	pedersenConstructorCalldata, err := crypto.PedersenArray(d.ConstructorCalldata...)
	if err != nil {
		return nil, err
	}
	data = append(data, pedersenConstructorCalldata)

	data = append(data, zeroFelt)

	chainIdFelt := new(felt.Felt).SetBytes(chainId)
	data = append(data, chainIdFelt)

	deployTransactionHash, err := crypto.PedersenArray(data...)
	if err != nil {
		return nil, err
	}
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
		pedersenHashCalldata, err := crypto.PedersenArray(i.CallData...)
		if err != nil {
			return nil, err
		}
		data = append(data, pedersenHashCalldata)

		data = append(data, i.MaxFee)

		chainIdFelt := new(felt.Felt).SetBytes(chainId)
		data = append(data, chainIdFelt)

		invokeTransactionHash, err := crypto.PedersenArray(data...)
		if err != nil {
			return nil, err
		}

		return invokeTransactionHash, nil
	} else if i.Version.IsOne() {
		// Implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v0_invoke_transaction
		// Transaction sender address
		senderAddress := i.SenderAddress
		data = append(data, senderAddress)

		// Zero Felt
		zeroFelt := new(felt.Felt).SetBytes([]byte("0"))
		data = append(data, zeroFelt)

		// Pedersen Hash of the Calldata
		pedersenHashCalldata, err := crypto.PedersenArray(i.CallData...)
		if err != nil {
			return nil, err
		}
		data = append(data, pedersenHashCalldata)

		data = append(data, i.MaxFee)

		chainIdFelt := new(felt.Felt).SetBytes(chainId)
		data = append(data, chainIdFelt)

		data = append(data, i.Nonce)

		invokeTransactionHash, err := crypto.PedersenArray(data...)
		if err != nil {
			return nil, err
		}

		return invokeTransactionHash, nil
	}
	return nil, errors.New("invalid transaction version")
}

type DeclareTransaction struct {
	// The class object.
	Class contract.Class
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

	declareFelt := new(felt.Felt).SetBytes([]byte("declare"))
	data = append(data, declareFelt)

	data = append(data, d.Version)

	// Sender Address
	senderAddress := d.SenderAddress
	data = append(data, senderAddress)

	// Zero Felt
	zeroFelt := new(felt.Felt).SetBytes([]byte("0"))
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
		classHash, err := d.Class.ClassHash()
		if err != nil {
			return nil, err
		}
		data = append(data, classHash)

		declareTransactionHash, err := crypto.PedersenArray(data...)
		if err != nil {
			return nil, err
		}

		return declareTransactionHash, nil
	} else if d.Version.IsOne() {
		// Todo: implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v1_declare_transaction
	}
	return nil, errors.New("invalid transaction version")
}

// func GenerateTransaction(transactionDefinition []byte) (DeployTransaction, error) {
// 	deploy := new(DeployTransaction)

// }

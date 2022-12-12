package core

import (
	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

type Transaction interface {
	Hash() *fp.Element
}

type DeployTransaction struct {
	// A random number used to distinguish between different instances of the contract.
	ContractAddressSalt *fp.Element
	// The object that defines the contract’s functionality.
	Class Class
	// The arguments passed to the constructor during deployment.
	ConstructorCalldata []*fp.Element
	// Who invoked the deployment. Set to 0 (in future: the deploying account contract).
	CallerAddress *fp.Element
	// The transaction’s version. Possible values are 1 or 0.
	//
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	// Transaction version 0 is deprecated and will be removed in a future version of StarkNet.
	Version *fp.Element
}

func (d *DeployTransaction) Hash() *fp.Element {
	// Todo: implement pedersen hash as defined here:
	// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_deploy_transaction
	return nil
}

type InvokeTransaction struct {
	// Version 0 fields
	// The address of the contract invoked by this transaction.
	ContractAddress *fp.Element
	// The encoding of the selector for the function invoked (the entry point in the contract)
	EntryPointSelector *fp.Element

	// Version 1 fields
	// The address of the sender of this transaction.
	SenderAddress *fp.Element
	// The transaction nonce.
	Nonce *fp.Element

	// The arguments that are passed to the validate and execute functions.
	CallData []*fp.Element
	// Additional information given by the sender, used to validate the transaction.
	Signature []*fp.Element
	// The maximum fee that the sender is willing to pay for the transaction
	MaxFee *fp.Element
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	Version *fp.Element
}

func (i *InvokeTransaction) Hash() *fp.Element {
	if i.Version.IsZero() {
		// Todo: implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v1_invoke_transaction
	} else if i.Version.IsOne() {
		// Todo: implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v0_invoke_transaction
	}
	return nil
}

type DeclareTransaction struct {
	// The class object.
	Class Class
	// The address of the account initiating the transaction.
	SenderAddress *fp.Element
	// The maximum fee that the sender is willing to pay for the transaction.
	MaxFee *fp.Element
	// Additional information given by the sender, used to validate the transaction.
	Signature []*fp.Element
	// The transaction nonce.
	Nonce *fp.Element
	// The transaction’s version. Possible values are 1 or 0.
	// When the fields that comprise a transaction change,
	// either with the addition of a new field or the removal of an existing field,
	// then the transaction version increases.
	// Transaction version 0 is deprecated and will be removed in a future version of StarkNet.
	Version *fp.Element
}

func (d *DeclareTransaction) Hash() *fp.Element {
	if d.Version.IsZero() {
		// Todo: implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v0_declare_transaction
	} else if d.Version.IsOne() {
		// Todo: implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v1_declare_transaction
	}
	return nil
}

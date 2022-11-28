package core

import (
	"github.com/NethermindEth/juno/core/contract"
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

func (d *DeployTransaction) Hash() *felt.Felt {
	// Todo: implement pedersen hash as defined here:
	// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_deploy_transaction
	return nil
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

func (i *InvokeTransaction) Hash() *felt.Felt {
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

func (d *DeclareTransaction) Hash() *felt.Felt {
	if d.Version.IsZero() {
		// Todo: implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v0_declare_transaction
	} else if d.Version.IsOne() {
		// Todo: implement pedersen hash as defined here:
		// https://docs.starknet.io/documentation/develop/Blocks/transactions/#calculating_the_hash_of_a_v1_declare_transaction
	}
	return nil
}

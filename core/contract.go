package core

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

// ContractAddress computes the address of a Starknet contract.
func ContractAddress(
	callerAddress,
	classHash,
	salt *felt.Felt,
	constructorCallData []*felt.Felt,
) felt.Felt {
	prefix := felt.FromBytes[felt.Felt]([]byte("STARKNET_CONTRACT_ADDRESS"))
	callDataHash := crypto.PedersenArray(constructorCallData...)

	// https://docs.starknet.io/architecture-and-concepts/smart-contracts/contract-address/
	return crypto.PedersenArray(
		&prefix,
		callerAddress,
		salt,
		classHash,
		&callDataHash,
	)
}

// ContractNonce returns the amount transactions sent from this contract.
// Only account contracts can have a non-zero nonce.
func ContractNonce(addr *felt.Felt, txn db.KeyValueReader) (felt.Felt, error) {
	return GetContractNonce(txn, addr)
}

// ContractClassHash returns hash of the class that the contract at the given address instantiates.
func ContractClassHash(addr *felt.Felt, txn db.KeyValueReader) (felt.Felt, error) {
	return GetContractClassHash(txn, addr)
}

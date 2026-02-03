//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
package testutils

import (
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	synctransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/transaction"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
)

type SyncTransactionBuilder[C, P any] struct {
	ToCore         toCoreType[C]
	ToP2PDeclareV0 toP2PType[P, *synctransaction.TransactionInBlock_DeclareV0WithoutClass]
	ToP2PDeclareV1 toP2PType[P, *synctransaction.TransactionInBlock_DeclareV1WithoutClass]
	ToP2PDeclareV2 toP2PType[P, *synctransaction.TransactionInBlock_DeclareV2WithoutClass]
	ToP2PDeclareV3 toP2PType[P, *synctransaction.TransactionInBlock_DeclareV3WithoutClass]
	ToP2PDeployV0  toP2PType[P, *synctransaction.TransactionInBlock_Deploy]
	ToP2PDeployV1  toP2PType[P, *synctransaction.TransactionInBlock_DeployAccountV1]
	ToP2PDeployV3  toP2PType[P, *synctransaction.TransactionInBlock_DeployAccountV3]
	ToP2PInvokeV0  toP2PType[P, *synctransaction.TransactionInBlock_InvokeV0]
	ToP2PInvokeV1  toP2PType[P, *synctransaction.TransactionInBlock_InvokeV1]
	ToP2PInvokeV3  toP2PType[P, *synctransaction.TransactionInBlock_InvokeV3]
	ToP2PL1Handler toP2PType[P, *synctransaction.TransactionInBlock_L1Handler]
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeclareV0Transaction(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	classHash, classHashBytes := getRandomFelt(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	maxFee, maxFeeBytes := getRandomFelt(t)
	version := new(core.TransactionVersion).SetUint64(0)

	p2pTransaction := synctransaction.TransactionInBlock_DeclareV0WithoutClass{
		Sender:    &common.Address{Elements: senderAddressBytes},
		MaxFee:    &common.Felt252{Elements: maxFeeBytes},
		Signature: &transaction.AccountSignature{Parts: toFelt252Slice(transactionSignatureBytes)},
		ClassHash: &common.Hash{Elements: classHashBytes},
	}

	consensusDeclareTransaction := core.DeclareTransaction{
		TransactionHash:       nil, // this field is populated later
		ClassHash:             &classHash,
		SenderAddress:         &senderAddress,
		MaxFee:                &maxFee,
		TransactionSignature:  transactionSignature,
		Nonce:                 nil, // this field is not available on v0
		Version:               version,
		CompiledClassHash:     nil, // this field is not available on v0
		ResourceBounds:        nil, // this field is not available on v0
		Tip:                   0,   // this field is not available on v0
		PaymasterData:         nil, // this field is not available on v0
		AccountDeploymentData: nil, // this field is not available on v0
		NonceDAMode:           0,   // this field is not available on v0
		FeeDAMode:             0,   // this field is not available on v0
	}

	transactionHash, p2pHash := getTransactionHash(
		t,
		&consensusDeclareTransaction,
		network,
	)
	consensusDeclareTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusDeclareTransaction, nil, nil),
		convertToP2P(b.ToP2PDeclareV0, &p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeclareV1Transaction(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()

	classHash, classHashBytes := getRandomFelt(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	nonce, nonceBytes := getRandomFelt(t)
	maxFee, maxFeeBytes := getRandomFelt(t)
	version := new(core.TransactionVersion).SetUint64(1)

	p2pTransaction := synctransaction.TransactionInBlock_DeclareV1WithoutClass{
		Sender:    &common.Address{Elements: senderAddressBytes},
		MaxFee:    &common.Felt252{Elements: maxFeeBytes},
		Signature: &transaction.AccountSignature{Parts: toFelt252Slice(transactionSignatureBytes)},
		ClassHash: &common.Hash{Elements: classHashBytes},
		Nonce:     &common.Felt252{Elements: nonceBytes},
	}

	consensusDeclareTransaction := core.DeclareTransaction{
		TransactionHash:       nil, // this field is populated later
		ClassHash:             &classHash,
		SenderAddress:         &senderAddress,
		MaxFee:                &maxFee,
		TransactionSignature:  transactionSignature,
		Nonce:                 &nonce,
		Version:               version,
		CompiledClassHash:     nil, // this field is not available on v1
		ResourceBounds:        nil, // this field is not available on v1
		PaymasterData:         nil, // this field is not available on v1
		AccountDeploymentData: nil, // this field is not available on v1
		Tip:                   0,   // this field is not available on v1
		NonceDAMode:           0,   // this field is not available on v1
		FeeDAMode:             0,   // this field is not available on v1
	}

	transactionHash, p2pHash := getTransactionHash(
		t,
		&consensusDeclareTransaction,
		network,
	)
	consensusDeclareTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusDeclareTransaction, nil, nil),
		convertToP2P(b.ToP2PDeclareV1, &p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeclareV2Transaction(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	classHash, classHashBytes := getRandomFelt(t)
	casmClassHash, casmClassHashBytes := getRandomFelt(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	nonce, nonceBytes := getRandomFelt(t)
	maxFee, maxFeeBytes := getRandomFelt(t)
	version := new(core.TransactionVersion).SetUint64(2)

	p2pTransaction := synctransaction.TransactionInBlock_DeclareV2WithoutClass{
		Sender: &common.Address{Elements: senderAddressBytes},
		MaxFee: &common.Felt252{Elements: maxFeeBytes},
		Signature: &transaction.AccountSignature{
			Parts: toFelt252Slice(transactionSignatureBytes),
		},
		ClassHash:         &common.Hash{Elements: classHashBytes},
		Nonce:             &common.Felt252{Elements: nonceBytes},
		CompiledClassHash: &common.Hash{Elements: casmClassHashBytes},
	}

	consensusDeclareTransaction := core.DeclareTransaction{
		TransactionHash:       nil, // this field is populated later
		ClassHash:             &classHash,
		SenderAddress:         &senderAddress,
		MaxFee:                &maxFee,
		TransactionSignature:  transactionSignature,
		Nonce:                 &nonce,
		Version:               version,
		CompiledClassHash:     &casmClassHash,
		ResourceBounds:        nil, // this field is not available on v2
		PaymasterData:         nil, // this field is not available on v2
		AccountDeploymentData: nil, // this field is not available on v2
		Tip:                   0,   // this field is not available on v2
		NonceDAMode:           0,   // this field is not available on v2
		FeeDAMode:             0,   // this field is not available on v2
	}

	transactionHash, p2pHash := getTransactionHash(
		t,
		&consensusDeclareTransaction,
		network,
	)
	consensusDeclareTransaction.TransactionHash = &transactionHash
	return b.ToCore(&consensusDeclareTransaction, nil, nil),
		convertToP2P(b.ToP2PDeclareV2, &p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeclareV3Transaction(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	classHash, classHashBytes := getRandomFelt(t)
	casmHash, casmHashBytes := getRandomFelt(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	nonce, nonceBytes := getRandomFelt(t)
	version := new(core.TransactionVersion).SetUint64(3)
	resourceBounds, p2pResourceBounds := getRandomResourceBounds(t)
	tip := rand.Uint64()
	paymasterData, paymasterDataBytes := getRandomFeltSlice(t)
	accountDeploymentData, accountDeploymentDataBytes := getRandomFeltSlice(t)

	p2pTransaction := synctransaction.TransactionInBlock_DeclareV3WithoutClass{
		Common: &transaction.DeclareV3Common{
			Sender: &common.Address{Elements: senderAddressBytes},
			Signature: &transaction.AccountSignature{
				Parts: toFelt252Slice(transactionSignatureBytes),
			},
			Nonce:                     &common.Felt252{Elements: nonceBytes},
			CompiledClassHash:         &common.Hash{Elements: casmHashBytes},
			ResourceBounds:            p2pResourceBounds,
			Tip:                       tip,
			PaymasterData:             toFelt252Slice(paymasterDataBytes),
			AccountDeploymentData:     toFelt252Slice(accountDeploymentDataBytes),
			NonceDataAvailabilityMode: common.VolitionDomain_L2,
			FeeDataAvailabilityMode:   common.VolitionDomain_L2,
		},
		ClassHash: &common.Hash{Elements: classHashBytes},
	}

	consensusDeclareTransaction := core.DeclareTransaction{
		TransactionHash:       nil, // this field is populated later
		ClassHash:             &classHash,
		SenderAddress:         &senderAddress,
		MaxFee:                nil, // this field is not available on v3
		TransactionSignature:  transactionSignature,
		Nonce:                 &nonce,
		Version:               version,
		CompiledClassHash:     &casmHash,
		ResourceBounds:        resourceBounds,
		Tip:                   tip,
		PaymasterData:         paymasterData,
		AccountDeploymentData: accountDeploymentData,
		NonceDAMode:           core.DAModeL2,
		FeeDAMode:             core.DAModeL2,
	}

	transactionHash, p2pHash := getTransactionHash(
		t,
		&consensusDeclareTransaction,
		network,
	)
	consensusDeclareTransaction.TransactionHash = &transactionHash
	return b.ToCore(&consensusDeclareTransaction, nil, nil),
		convertToP2P(b.ToP2PDeclareV3, &p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeployTransactionV0(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	// DeployTransaction is a special case because we don't verify the transaction hash
	// so we can set it to a random value
	txHash, txHashBytes := getRandomFelt(t)
	contractAddressSalt, contractAddressSaltBytes := getRandomFelt(t)
	classHash, classHashBytes := getRandomFelt(t)
	constructorCallData, constructorCallDataBytes := getRandomFeltSlice(t)
	contractAddress := core.ContractAddress(
		&felt.Zero,
		&classHash,
		&contractAddressSalt,
		constructorCallData,
	)
	version := new(core.TransactionVersion).SetUint64(0)

	p2pTransaction := synctransaction.TransactionInBlock_Deploy{
		ClassHash:   &common.Hash{Elements: classHashBytes},
		AddressSalt: &common.Felt252{Elements: contractAddressSaltBytes},
		Calldata:    toFelt252Slice(constructorCallDataBytes),
		Version:     0, // todo(kirill) remove field from spec? tx is deprecated so no future versions
	}

	consensusDeployTransaction := core.DeployTransaction{
		TransactionHash:     &txHash,
		ContractAddress:     &contractAddress,
		ContractAddressSalt: &contractAddressSalt,
		ClassHash:           &classHash,
		ConstructorCallData: constructorCallData,
		Version:             version,
	}

	return b.ToCore(&consensusDeployTransaction, nil, nil),
		convertToP2P(b.ToP2PDeployV0, &p2pTransaction, &common.Hash{Elements: txHashBytes})
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeployAccountTransactionV1(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	contractAddressSalt, contractAddressSaltBytes := getRandomFelt(t)
	classHash, classHashBytes := getRandomFelt(t)
	constructorCallData, constructorCallDataBytes := getRandomFeltSlice(t)
	contractAddress := core.ContractAddress(
		&felt.Zero, &classHash,
		&contractAddressSalt,
		constructorCallData,
	)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	maxFee, maxFeeBytes := getRandomFelt(t)
	nonce, nonceBytes := getRandomFelt(t)
	version := new(core.TransactionVersion).SetUint64(1)

	p2pTransaction := synctransaction.TransactionInBlock_DeployAccountV1{
		MaxFee: &common.Felt252{Elements: maxFeeBytes},
		Signature: &transaction.AccountSignature{
			Parts: toFelt252Slice(transactionSignatureBytes),
		},
		ClassHash:   &common.Hash{Elements: classHashBytes},
		Nonce:       &common.Felt252{Elements: nonceBytes},
		AddressSalt: &common.Felt252{Elements: contractAddressSaltBytes},
		Calldata:    toFelt252Slice(constructorCallDataBytes),
	}

	consensusDeployAccountTransaction := core.DeployAccountTransaction{
		DeployTransaction: core.DeployTransaction{
			TransactionHash:     nil, // this field is populated later
			ContractAddressSalt: &contractAddressSalt,
			ContractAddress:     &contractAddress,
			ClassHash:           &classHash,
			ConstructorCallData: constructorCallData,
			Version:             version,
		},
		MaxFee:               &maxFee,
		TransactionSignature: transactionSignature,
		Nonce:                &nonce,
		ResourceBounds:       nil, // this field is not available on v1
		PaymasterData:        nil, // this field is not available on v1
		Tip:                  0,   // this field is not available on v1
		NonceDAMode:          0,   // this field is not available on v1
		FeeDAMode:            0,   // this field is not available on v1
	}

	transactionHash, p2pHash := getTransactionHash(
		t,
		&consensusDeployAccountTransaction,
		network,
	)
	consensusDeployAccountTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusDeployAccountTransaction, nil, nil),
		convertToP2P(b.ToP2PDeployV1, &p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeployAccountTransactionV3(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	contractAddressSalt, contractAddressSaltBytes := getRandomFelt(t)
	classHash, classHashBytes := getRandomFelt(t)
	constructorCallData, constructorCallDataBytes := getRandomFeltSlice(t)
	contractAddress := core.ContractAddress(
		&felt.Zero, &classHash,
		&contractAddressSalt,
		constructorCallData,
	)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	nonce, nonceBytes := getRandomFelt(t)
	version := new(core.TransactionVersion).SetUint64(3)
	tip := rand.Uint64()
	resourceBounds, p2pResourceBounds := getRandomResourceBounds(t)
	paymasterData, paymasterDataBytes := getRandomFeltSlice(t)

	p2pTransaction := synctransaction.TransactionInBlock_DeployAccountV3{
		DeployAccountV3: &transaction.DeployAccountV3{
			Signature: &transaction.AccountSignature{
				Parts: toFelt252Slice(transactionSignatureBytes),
			},
			ClassHash:                 &common.Hash{Elements: classHashBytes},
			Nonce:                     &common.Felt252{Elements: nonceBytes},
			AddressSalt:               &common.Felt252{Elements: contractAddressSaltBytes},
			Calldata:                  toFelt252Slice(constructorCallDataBytes),
			ResourceBounds:            p2pResourceBounds,
			Tip:                       tip,
			PaymasterData:             toFelt252Slice(paymasterDataBytes),
			NonceDataAvailabilityMode: common.VolitionDomain_L2,
			FeeDataAvailabilityMode:   common.VolitionDomain_L2,
		},
	}

	consensusDeployAccountTransaction := core.DeployAccountTransaction{
		DeployTransaction: core.DeployTransaction{
			TransactionHash:     nil, // this field is populated later
			ContractAddressSalt: &contractAddressSalt,
			ContractAddress:     &contractAddress,
			ClassHash:           &classHash,
			ConstructorCallData: constructorCallData,
			Version:             version,
		},
		MaxFee:               nil, // Unused field
		TransactionSignature: transactionSignature,
		Nonce:                &nonce,
		ResourceBounds:       resourceBounds,
		Tip:                  tip,
		PaymasterData:        paymasterData,
		NonceDAMode:          core.DAModeL2,
		FeeDAMode:            core.DAModeL2,
	}

	transactionHash, p2pHash := getTransactionHash(
		t,
		&consensusDeployAccountTransaction,
		network,
	)
	consensusDeployAccountTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusDeployAccountTransaction, nil, nil),
		convertToP2P(b.ToP2PDeployV3, &p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestInvokeTransactionV0(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	constructorCallData, constructorCallDataBytes := getRandomFeltSlice(t)
	contractAddress, contractAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	version := new(core.TransactionVersion).SetUint64(0)
	maxFee, maxFeeBytes := getRandomFelt(t)
	entryPointSelector, entryPointSelectorBytes := getRandomFelt(t)

	p2pTransaction := synctransaction.TransactionInBlock_InvokeV0{
		MaxFee: &common.Felt252{Elements: maxFeeBytes},
		Signature: &transaction.AccountSignature{
			Parts: toFelt252Slice(transactionSignatureBytes),
		},
		Address:            &common.Address{Elements: contractAddressBytes},
		EntryPointSelector: &common.Felt252{Elements: entryPointSelectorBytes},
		Calldata:           toFelt252Slice(constructorCallDataBytes),
	}

	consensusDeployAccountTransaction := core.InvokeTransaction{
		TransactionHash:       nil, // this field is populated later
		CallData:              constructorCallData,
		TransactionSignature:  transactionSignature,
		MaxFee:                &maxFee,
		ContractAddress:       &contractAddress,
		Version:               version,
		EntryPointSelector:    &entryPointSelector,
		Nonce:                 nil, // this field is not available on v0
		SenderAddress:         nil, // this field is not available on v0
		ResourceBounds:        nil, // this field is not available on v0
		Tip:                   0,   // this field is not available on v0
		PaymasterData:         nil, // this field is not available on v0
		AccountDeploymentData: nil, // this field is not available on v0
		NonceDAMode:           0,   // this field is not available on v0
		FeeDAMode:             0,   // this field is not available on v0
	}

	transactionHash, p2pHash := getTransactionHash(
		t,
		&consensusDeployAccountTransaction,
		network,
	)
	consensusDeployAccountTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusDeployAccountTransaction, nil, nil),
		convertToP2P(b.ToP2PInvokeV0, &p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestInvokeTransactionV1(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	constructorCallData, constructorCallDataBytes := getRandomFeltSlice(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	version := new(core.TransactionVersion).SetUint64(1)
	nonce, nonceBytes := getRandomFelt(t)
	maxFee, maxFeeBytes := getRandomFelt(t)

	p2pTransaction := synctransaction.TransactionInBlock_InvokeV1{
		Sender: &common.Address{Elements: senderAddressBytes},
		MaxFee: &common.Felt252{Elements: maxFeeBytes},
		Signature: &transaction.AccountSignature{
			Parts: toFelt252Slice(transactionSignatureBytes),
		},
		Calldata: toFelt252Slice(constructorCallDataBytes),
		Nonce:    &common.Felt252{Elements: nonceBytes},
	}

	consensusDeployAccountTransaction := core.InvokeTransaction{
		TransactionHash:       nil, // This field is populated later
		ContractAddress:       nil, // todo call core.ContractAddress() ?
		Nonce:                 &nonce,
		SenderAddress:         &senderAddress,
		CallData:              constructorCallData,
		TransactionSignature:  transactionSignature,
		MaxFee:                &maxFee,
		Version:               version,
		EntryPointSelector:    nil, // this field is not available on v1
		ResourceBounds:        nil, // this field is not available on v1
		Tip:                   0,   // this field is not available on v1
		PaymasterData:         nil, // this field is not available on v1
		AccountDeploymentData: nil, // this field is not available on v1
		NonceDAMode:           0,   // this field is not available on v1
		FeeDAMode:             0,   // this field is not available on v1
	}

	transactionHash, p2pHash := getTransactionHash(
		t,
		&consensusDeployAccountTransaction,
		network,
	)
	consensusDeployAccountTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusDeployAccountTransaction, nil, nil),
		convertToP2P(b.ToP2PInvokeV1, &p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestInvokeTransactionV3(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	constructorCallData, constructorCallDataBytes := getRandomFeltSlice(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	version := new(core.TransactionVersion).SetUint64(3)
	nonce, nonceBytes := getRandomFelt(t)
	tip := rand.Uint64()
	resourceBounds, p2pResourceBounds := getRandomResourceBounds(t)
	paymasterData, paymasterDataBytes := getRandomFeltSlice(t)

	p2pTransaction := synctransaction.TransactionInBlock_InvokeV3{
		InvokeV3: &transaction.InvokeV3{
			Sender: &common.Address{Elements: senderAddressBytes},
			Signature: &transaction.AccountSignature{
				Parts: toFelt252Slice(transactionSignatureBytes),
			},
			Calldata:                  toFelt252Slice(constructorCallDataBytes),
			ResourceBounds:            p2pResourceBounds,
			Tip:                       tip,
			PaymasterData:             toFelt252Slice(paymasterDataBytes),
			AccountDeploymentData:     nil, // TODO: this is for future use as per starknet document
			NonceDataAvailabilityMode: common.VolitionDomain_L2,
			FeeDataAvailabilityMode:   common.VolitionDomain_L2,
			Nonce:                     &common.Felt252{Elements: nonceBytes},
		},
	}

	consensusDeployAccountTransaction := core.InvokeTransaction{
		TransactionHash:       nil, // this field is populated later
		ContractAddress:       nil, // todo call core.ContractAddress() ?
		CallData:              constructorCallData,
		TransactionSignature:  transactionSignature,
		MaxFee:                nil, // in 3 version this field was removed
		Version:               version,
		Nonce:                 &nonce,
		SenderAddress:         &senderAddress,
		EntryPointSelector:    nil, // TODO: Figure out why the original adapter doesn't set this field
		Tip:                   tip,
		ResourceBounds:        resourceBounds,
		PaymasterData:         paymasterData,
		NonceDAMode:           core.DAModeL2,
		FeeDAMode:             core.DAModeL2,
		AccountDeploymentData: nil, // TODO: this is for future use as per starknet document
	}

	transactionHash, p2pHash := getTransactionHash(
		t,
		&consensusDeployAccountTransaction,
		network,
	)
	consensusDeployAccountTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusDeployAccountTransaction, nil, nil),
		convertToP2P(b.ToP2PInvokeV3, &p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestL1HandlerTransaction(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	contractAddress, contractAddressBytes := getRandomFelt(t)
	entryPointSelector, entryPointSelectorBytes := getRandomFelt(t)
	nonce, nonceBytes := getRandomFelt(t)
	callData, callDataBytes := getRandomFeltSlice(t)
	version := new(core.TransactionVersion).SetUint64(0)

	p2pTransaction := synctransaction.TransactionInBlock_L1Handler{
		L1Handler: &transaction.L1HandlerV0{
			Nonce:              &common.Felt252{Elements: nonceBytes},
			Address:            &common.Address{Elements: contractAddressBytes},
			EntryPointSelector: &common.Felt252{Elements: entryPointSelectorBytes},
			Calldata:           toFelt252Slice(callDataBytes),
		},
	}

	consensusL1HandlerTransaction := core.L1HandlerTransaction{
		TransactionHash:    nil, // This field is populated later
		ContractAddress:    &contractAddress,
		EntryPointSelector: &entryPointSelector,
		Nonce:              &nonce,
		CallData:           callData,
		Version:            version,
	}

	transactionHash, p2pHash := getTransactionHash(
		t,
		&consensusL1HandlerTransaction,
		network,
	)
	consensusL1HandlerTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusL1HandlerTransaction, nil, felt.One.Clone()),
		convertToP2P(b.ToP2PL1Handler, &p2pTransaction, p2pHash)
}

// Allows to ignore the conversion function if it's nil. This is useful when we just want the core
// type generators.
func convertToP2P[P, I any](f func(I, *common.Hash) P, inner I, hash *common.Hash) P {
	var p P
	if f != nil {
		p = f(inner, hash)
	}
	return p
}

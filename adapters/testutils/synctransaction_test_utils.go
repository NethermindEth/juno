//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
package testutils

import (
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/adapters/core2p2p"
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
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeclareV0Transaction(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	classHash, _ := getSampleClass(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	maxFee, maxFeeBytes := getRandomFelt(t)
	version := new(core.TransactionVersion).SetUint64(0)

	p2pTransaction := synctransaction.TransactionInBlock_DeclareV0WithoutClass{
		Sender:    &common.Address{Elements: senderAddressBytes},
		MaxFee:    &common.Felt252{Elements: maxFeeBytes},
		Signature: &transaction.AccountSignature{Parts: toFelt252Slice(transactionSignatureBytes)},
		ClassHash: core2p2p.AdaptHash(&classHash),
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

	var p2pHash *common.Hash
	consensusDeclareTransaction.TransactionHash, p2pHash = getTransactionHash(
		t,
		&consensusDeclareTransaction,
		network,
	)
	return b.ToCore(&consensusDeclareTransaction, nil, nil),
		b.ToP2PDeclareV0(&p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeclareV1Transaction(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()

	classHash, _ := getSampleClass(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	nonce, nonceBytes := getRandomFelt(t)
	maxFee, maxFeeBytes := getRandomFelt(t)
	version := new(core.TransactionVersion).SetUint64(1)

	p2pTransaction := synctransaction.TransactionInBlock_DeclareV1WithoutClass{
		Sender:    &common.Address{Elements: senderAddressBytes},
		MaxFee:    &common.Felt252{Elements: maxFeeBytes},
		Signature: &transaction.AccountSignature{Parts: toFelt252Slice(transactionSignatureBytes)},
		ClassHash: core2p2p.AdaptHash(&classHash),
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

	var p2pHash *common.Hash
	consensusDeclareTransaction.TransactionHash, p2pHash = getTransactionHash(
		t,
		&consensusDeclareTransaction,
		network,
	)
	return b.ToCore(&consensusDeclareTransaction, nil, nil),
		b.ToP2PDeclareV1(&p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeclareV2Transaction(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	classHash, cairo1Class := getSampleClass(t)
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
		ClassHash:         core2p2p.AdaptHash(&classHash),
		Nonce:             &common.Felt252{Elements: nonceBytes},
		CompiledClassHash: core2p2p.AdaptHash(cairo1Class.Compiled.Hash()),
	}

	consensusDeclareTransaction := core.DeclareTransaction{
		TransactionHash:       nil, // this field is populated later
		ClassHash:             &classHash,
		SenderAddress:         &senderAddress,
		MaxFee:                &maxFee,
		TransactionSignature:  transactionSignature,
		Nonce:                 &nonce,
		Version:               version,
		CompiledClassHash:     cairo1Class.Compiled.Hash(),
		ResourceBounds:        nil, // this field is not available on v2
		PaymasterData:         nil, // this field is not available on v2
		AccountDeploymentData: nil, // this field is not available on v2
		Tip:                   0,   // this field is not available on v2
		NonceDAMode:           0,   // this field is not available on v2
		FeeDAMode:             0,   // this field is not available on v2
	}

	var p2pHash *common.Hash
	consensusDeclareTransaction.TransactionHash, p2pHash = getTransactionHash(
		t,
		&consensusDeclareTransaction,
		network,
	)
	return b.ToCore(&consensusDeclareTransaction, nil, nil),
		b.ToP2PDeclareV2(&p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeclareV3Transaction(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	classHash, cairo1Class := getSampleClass(t)
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
			CompiledClassHash:         core2p2p.AdaptHash(cairo1Class.Compiled.Hash()),
			ResourceBounds:            p2pResourceBounds,
			Tip:                       tip,
			PaymasterData:             toFelt252Slice(paymasterDataBytes),
			AccountDeploymentData:     toFelt252Slice(accountDeploymentDataBytes),
			NonceDataAvailabilityMode: common.VolitionDomain_L2,
			FeeDataAvailabilityMode:   common.VolitionDomain_L2,
		},
		ClassHash: core2p2p.AdaptHash(&classHash),
	}

	consensusDeclareTransaction := core.DeclareTransaction{
		TransactionHash:       nil, // this field is populated later
		ClassHash:             &classHash,
		SenderAddress:         &senderAddress,
		MaxFee:                nil, // this field is not available on v3
		TransactionSignature:  transactionSignature,
		Nonce:                 &nonce,
		Version:               version,
		CompiledClassHash:     cairo1Class.Compiled.Hash(),
		ResourceBounds:        resourceBounds,
		Tip:                   tip,
		PaymasterData:         paymasterData,
		AccountDeploymentData: accountDeploymentData,
		NonceDAMode:           core.DAModeL2,
		FeeDAMode:             core.DAModeL2,
	}

	var p2pHash *common.Hash
	consensusDeclareTransaction.TransactionHash, p2pHash = getTransactionHash(
		t,
		&consensusDeclareTransaction,
		network,
	)
	return b.ToCore(&consensusDeclareTransaction, nil, nil),
		b.ToP2PDeclareV3(&p2pTransaction, p2pHash)
}

func (b *SyncTransactionBuilder[C, P]) GetTestDeployTransactionV0(
	t *testing.T,
	network *utils.Network,
) (C, P) {
	t.Helper()
	contractAddressSalt, contractAddressSaltBytes := getRandomFelt(t)
	classHash, _ := getRandomFelt(t)
	constructorCallData, _ := getRandomFeltSlice(t)
	contractAddress := core.ContractAddress(
		&felt.Zero, &classHash,
		&contractAddressSalt,
		constructorCallData,
	)
	version := new(core.TransactionVersion).SetUint64(0)

	p2pTransaction := synctransaction.TransactionInBlock_Deploy{
		ClassHash:   core2p2p.AdaptHash(&classHash),
		AddressSalt: &common.Felt252{Elements: contractAddressSaltBytes},
		Calldata:    utils.Map(constructorCallData, core2p2p.AdaptFelt),
		Version:     0, // todo(kirill) remove field from spec? tx is deprecated so no future versions
	}

	consensusDeployTransaction := core.DeployTransaction{
		TransactionHash:     nil,
		ContractAddress:     contractAddress,
		ContractAddressSalt: &contractAddressSalt,
		ClassHash:           &classHash,
		ConstructorCallData: constructorCallData,
		Version:             version,
	}
	var p2pHash *common.Hash
	consensusDeployTransaction.TransactionHash, p2pHash = getTransactionHash(
		t,
		&consensusDeployTransaction,
		network,
	)
	return b.ToCore(&consensusDeployTransaction, nil, nil),
		b.ToP2PDeployV0(&p2pTransaction, p2pHash)
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
			TransactionHash:     nil,
			ContractAddressSalt: &contractAddressSalt,
			ContractAddress:     contractAddress,
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
	var p2pHash *common.Hash
	consensusDeployAccountTransaction.TransactionHash, p2pHash = getTransactionHash(
		t,
		&consensusDeployAccountTransaction,
		network,
	)
	return b.ToCore(&consensusDeployAccountTransaction, nil, nil),
		b.ToP2PDeployV1(&p2pTransaction, p2pHash)
}

//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
package testutils

import (
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
	"github.com/stretchr/testify/require"
)

const maxTransactionSize = 10

type toCoreType[C any] func(core.Transaction, core.ClassDefinition, *felt.Felt) C

type toP2PType[P, I any] func(I, *common.Hash) P

type TransactionBuilder[C, P any] struct {
	ToCore         toCoreType[C]
	ToP2PDeclareV3 toP2PType[P, *transaction.DeclareV3WithClass]
	ToP2PDeploy    toP2PType[P, *transaction.DeployAccountV3]
	ToP2PInvoke    toP2PType[P, *transaction.InvokeV3]
	ToP2PL1Handler toP2PType[P, *transaction.L1HandlerV0]
}

type factory[C, P any] func(t *testing.T, network *utils.Network) (C, P)

func GetTestTransactions[C, P any](t *testing.T, network *utils.Network, factories ...factory[C, P]) ([]C, []P) {
	t.Helper()
	consensusTransactions := make([]C, len(factories))
	p2pTransactions := make([]P, len(factories))

	for i := range factories {
		consensusTransactions[i], p2pTransactions[i] = factories[i](t, network)
	}

	return consensusTransactions, p2pTransactions
}

func getRandomFelt(t *testing.T) (felt.Felt, []byte) {
	t.Helper()

	f := felt.Random[felt.Felt]()
	feltBytes := f.Bytes()
	return f, feltBytes[:]
}

func getRandomFeltSlice(t *testing.T) ([]*felt.Felt, [][]byte) {
	t.Helper()

	size := rand.IntN(maxTransactionSize)
	underlyingFelts := make([]felt.Felt, size)
	felts := make([]*felt.Felt, size)
	feltBytes := make([][]byte, size)

	for i := range size {
		underlyingFelts[i], feltBytes[i] = getRandomFelt(t)
		felts[i] = &underlyingFelts[i]
	}

	return felts, feltBytes
}

func toFelt252Slice(felts [][]byte) []*common.Felt252 {
	felt252s := make([]*common.Felt252, len(felts))
	for i := range felts {
		felt252s[i] = &common.Felt252{Elements: felts[i]}
	}
	return felt252s
}

func getRandomResourceLimits(t *testing.T) (core.ResourceBounds, *transaction.ResourceLimits) {
	t.Helper()
	maxAmount := rand.Uint64()
	maxAmountFelt := felt.FromUint64[felt.Felt](maxAmount)
	maxAmountFeltBytes := maxAmountFelt.Bytes()
	maxPricePerUnit, maxPricePerUnitBytes := getRandomFelt(t)

	resourceBounds := core.ResourceBounds{
		MaxAmount:       maxAmount,
		MaxPricePerUnit: &maxPricePerUnit,
	}

	resourceLimits := transaction.ResourceLimits{
		MaxAmount:       &common.Felt252{Elements: maxAmountFeltBytes[:]},
		MaxPricePerUnit: &common.Felt252{Elements: maxPricePerUnitBytes},
	}

	return resourceBounds, &resourceLimits
}

func getRandomResourceBounds(t *testing.T) (map[core.Resource]core.ResourceBounds, *transaction.ResourceBounds) {
	t.Helper()
	consensusL1ResourceLimits, p2pL1ResourceLimits := getRandomResourceLimits(t)
	consensusL2ResourceLimits, p2pL2ResourceLimits := getRandomResourceLimits(t)
	consensusL1DataResourceLimits, p2pL1DataResourceLimits := getRandomResourceLimits(t)

	consensusResourceBounds := map[core.Resource]core.ResourceBounds{
		core.ResourceL1Gas:     consensusL1ResourceLimits,
		core.ResourceL2Gas:     consensusL2ResourceLimits,
		core.ResourceL1DataGas: consensusL1DataResourceLimits,
	}

	p2pResourceBounds := transaction.ResourceBounds{
		L1Gas:     p2pL1ResourceLimits,
		L2Gas:     p2pL2ResourceLimits,
		L1DataGas: p2pL1DataResourceLimits,
	}

	return consensusResourceBounds, &p2pResourceBounds
}

func getSampleClass(t *testing.T) (felt.Felt, *core.SierraClass) {
	t.Helper()
	var classHash felt.Felt
	_, err := classHash.SetString("0x3cc90db763e736ca9b6c581ea4008408842b1a125947ab087438676a7e40b7b")
	require.NoError(t, err)

	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	class, err := gw.Class(t.Context(), &classHash)
	require.NoError(t, err)

	sierraClass, ok := class.(*core.SierraClass)
	require.True(t, ok)

	return classHash, sierraClass
}

func getTransactionHash(
	t *testing.T,
	tx core.Transaction,
	network *utils.Network,
) (felt.Felt, *common.Hash) {
	t.Helper()
	hash, err := core.TransactionHash(tx, network)
	require.NoError(t, err)
	adaptedHash := core2p2p.AdaptHash(&hash)
	return hash, adaptedHash
}

func (b *TransactionBuilder[C, P]) GetTestDeclareTransaction(t *testing.T, network *utils.Network) (C, P) {
	t.Helper()
	classHash, sierraClass := getSampleClass(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	nonce, nonceBytes := getRandomFelt(t)
	version := new(core.TransactionVersion).SetUint64(3)
	resourceBounds, p2pResourceBounds := getRandomResourceBounds(t)
	tip := rand.Uint64()
	paymasterData, paymasterDataBytes := getRandomFeltSlice(t)
	accountDeploymentData, accountDeploymentDataBytes := getRandomFeltSlice(t)
	casmHash := sierraClass.Compiled.Hash(core.HashVersionV1)

	consensusDeclareTransaction := core.DeclareTransaction{
		TransactionHash:       nil,
		ClassHash:             &classHash,
		SenderAddress:         &senderAddress,
		MaxFee:                nil, // Unused field
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

	p2pTransaction := transaction.DeclareV3WithClass{
		Common: &transaction.DeclareV3Common{
			Sender:                    &common.Address{Elements: senderAddressBytes},
			Signature:                 &transaction.AccountSignature{Parts: toFelt252Slice(transactionSignatureBytes)},
			Nonce:                     &common.Felt252{Elements: nonceBytes},
			CompiledClassHash:         core2p2p.AdaptHash(&casmHash),
			ResourceBounds:            p2pResourceBounds,
			Tip:                       tip,
			PaymasterData:             toFelt252Slice(paymasterDataBytes),
			AccountDeploymentData:     toFelt252Slice(accountDeploymentDataBytes),
			NonceDataAvailabilityMode: common.VolitionDomain_L2,
			FeeDataAvailabilityMode:   common.VolitionDomain_L2,
		},
		Class: core2p2p.AdaptSierraClass(sierraClass),
	}

	transactionHash, p2pHash := getTransactionHash(t, &consensusDeclareTransaction, network)
	consensusDeclareTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusDeclareTransaction, sierraClass, nil),
		b.ToP2PDeclareV3(&p2pTransaction, p2pHash)
}

func (b *TransactionBuilder[C, P]) GetTestDeployAccountTransaction(t *testing.T, network *utils.Network) (C, P) {
	t.Helper()
	contractAddressSalt, contractAddressSaltBytes := getRandomFelt(t)
	classHash, classHashBytes := getRandomFelt(t)
	constructorCallData, constructorCallDataBytes := getRandomFeltSlice(t)
	version := new(core.TransactionVersion).SetUint64(3)
	contractAddress := core.ContractAddress(&felt.Zero, &classHash, &contractAddressSalt, constructorCallData)

	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	nonce, nonceBytes := getRandomFelt(t)
	resourceBounds, p2pResourceBounds := getRandomResourceBounds(t)
	tip := rand.Uint64()
	paymasterData, paymasterDataBytes := getRandomFeltSlice(t)

	consensusDeployAccountTransaction := core.DeployAccountTransaction{
		DeployTransaction: core.DeployTransaction{
			TransactionHash:     nil,
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

	p2pTransaction := transaction.DeployAccountV3{
		Signature:                 &transaction.AccountSignature{Parts: toFelt252Slice(transactionSignatureBytes)},
		ClassHash:                 &common.Hash{Elements: classHashBytes},
		Nonce:                     &common.Felt252{Elements: nonceBytes},
		AddressSalt:               &common.Felt252{Elements: contractAddressSaltBytes},
		Calldata:                  toFelt252Slice(constructorCallDataBytes),
		ResourceBounds:            p2pResourceBounds,
		Tip:                       tip,
		PaymasterData:             toFelt252Slice(paymasterDataBytes),
		NonceDataAvailabilityMode: common.VolitionDomain_L2,
		FeeDataAvailabilityMode:   common.VolitionDomain_L2,
	}

	transactionHash, p2pHash := getTransactionHash(t, &consensusDeployAccountTransaction, network)
	consensusDeployAccountTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusDeployAccountTransaction, nil, nil), b.ToP2PDeploy(&p2pTransaction, p2pHash)
}

func (b *TransactionBuilder[C, P]) GetTestInvokeTransaction(t *testing.T, network *utils.Network) (C, P) {
	t.Helper()
	constructorCallData, constructorCallDataBytes := getRandomFeltSlice(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	version := new(core.TransactionVersion).SetUint64(3)
	nonce, nonceBytes := getRandomFelt(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	resourceBounds, p2pResourceBounds := getRandomResourceBounds(t)
	tip := rand.Uint64()
	paymasterData, paymasterDataBytes := getRandomFeltSlice(t)

	consensusInvokeTransaction := core.InvokeTransaction{
		TransactionHash:       nil,
		CallData:              constructorCallData,
		TransactionSignature:  transactionSignature,
		MaxFee:                nil, // Unused field
		ContractAddress:       nil, // TODO: Figure out why the original adapter doesn't set this field
		Version:               version,
		EntryPointSelector:    nil, // TODO: Figure out why the original adapter doesn't set this field
		Nonce:                 &nonce,
		SenderAddress:         &senderAddress,
		ResourceBounds:        resourceBounds,
		Tip:                   tip,
		PaymasterData:         paymasterData,
		AccountDeploymentData: nil, // TODO: Figure out why the original adapter doesn't set this field
		NonceDAMode:           core.DAModeL2,
		FeeDAMode:             core.DAModeL2,
	}

	p2pTransaction := transaction.InvokeV3{
		Sender:                    &common.Address{Elements: senderAddressBytes},
		Signature:                 &transaction.AccountSignature{Parts: toFelt252Slice(transactionSignatureBytes)},
		Calldata:                  toFelt252Slice(constructorCallDataBytes),
		ResourceBounds:            p2pResourceBounds,
		Tip:                       tip,
		PaymasterData:             toFelt252Slice(paymasterDataBytes),
		AccountDeploymentData:     nil, // TODO: Figure out why the original adapter doesn't set this field
		NonceDataAvailabilityMode: common.VolitionDomain_L2,
		FeeDataAvailabilityMode:   common.VolitionDomain_L2,
		Nonce:                     &common.Felt252{Elements: nonceBytes},
	}

	transactionHash, p2pHash := getTransactionHash(t, &consensusInvokeTransaction, network)
	consensusInvokeTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusInvokeTransaction, nil, nil), b.ToP2PInvoke(&p2pTransaction, p2pHash)
}

func (b *TransactionBuilder[C, P]) GetTestL1HandlerTransaction(t *testing.T, network *utils.Network) (C, P) {
	t.Helper()
	contractAddress, contractAddressBytes := getRandomFelt(t)
	entryPointSelector, entryPointSelectorBytes := getRandomFelt(t)
	nonce, nonceBytes := getRandomFelt(t)
	callData, callDataBytes := getRandomFeltSlice(t)
	version := new(core.TransactionVersion).SetUint64(0)

	consensusL1HandlerTransaction := core.L1HandlerTransaction{
		TransactionHash:    nil,
		ContractAddress:    &contractAddress,
		EntryPointSelector: &entryPointSelector,
		Nonce:              &nonce,
		CallData:           callData,
		Version:            version,
	}

	p2pTransaction := transaction.L1HandlerV0{
		Nonce:              &common.Felt252{Elements: nonceBytes},
		Address:            &common.Address{Elements: contractAddressBytes},
		EntryPointSelector: &common.Felt252{Elements: entryPointSelectorBytes},
		Calldata:           toFelt252Slice(callDataBytes),
	}

	transactionHash, p2pHash := getTransactionHash(t, &consensusL1HandlerTransaction, network)
	consensusL1HandlerTransaction.TransactionHash = &transactionHash

	return b.ToCore(&consensusL1HandlerTransaction, nil, felt.One.Clone()), b.ToP2PL1Handler(&p2pTransaction, p2pHash)
}

// StripCompilerFields strips the some fields related to compiler in the compiled class.
// It's due to the difference between the expected compiler version and the actual compiler version.
func StripCompilerFields(t *testing.T, class core.ClassDefinition) {
	t.Helper()
	switch class := class.(type) {
	case *core.SierraClass:
		if class == nil {
			return
		}

		compilerVersion, hints, pythonicHints := class.Compiled.CompilerVersion, class.Compiled.Hints, class.Compiled.PythonicHints
		class.Compiled.CompilerVersion = ""
		class.Compiled.Hints = nil
		class.Compiled.PythonicHints = nil
		t.Cleanup(func() {
			class.Compiled.CompilerVersion = compilerVersion
			class.Compiled.Hints = hints
			class.Compiled.PythonicHints = pythonicHints
		})
	default:
		return
	}
}

// Allows to ignore the conversion function if it's nil. This is useful when we just want the core
// type generators.
func ConvertToP2P[P, I any](f func(I, *common.Hash) P, inner I, hash *common.Hash) P {
	var p P
	if f != nil {
		p = f(inner, hash)
	}
	return p
}

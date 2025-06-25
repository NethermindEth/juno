//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
package testutils

import (
	"math/rand/v2"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/clients/feeder"
	consensus "github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	p2pconsensus "github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
	"github.com/stretchr/testify/require"
)

const maxTransactionSize = 10

type TransactionFactory func(t *testing.T) (consensus.Transaction, *p2pconsensus.ConsensusTransaction)

func GetTestTransactions(t *testing.T, factories []TransactionFactory) ([]consensus.Transaction, []*p2pconsensus.ConsensusTransaction) {
	consensusTransactions := make([]consensus.Transaction, len(factories))
	p2pTransactions := make([]*p2pconsensus.ConsensusTransaction, len(factories))

	for i := range factories {
		consensusTransactions[i], p2pTransactions[i] = factories[i](t)
	}

	return consensusTransactions, p2pTransactions
}

func getRandomFelt(t *testing.T) (felt.Felt, []byte) {
	felt := felt.Felt{}
	_, err := felt.SetRandom()
	require.NoError(t, err)

	feltBytes := felt.Bytes()
	return felt, feltBytes[:]
}

func getRandomFeltSlice(t *testing.T) ([]*felt.Felt, [][]byte) {
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

func getRandomUint128() (*felt.Felt, *common.Uint128) {
	low := rand.Uint64()
	high := rand.Uint64()
	uint128 := common.Uint128{
		Low:  low,
		High: high,
	}
	felt := p2p2core.AdaptUint128(&uint128)
	return felt, &uint128
}

func toFelt252Slice(felts [][]byte) []*common.Felt252 {
	felt252s := make([]*common.Felt252, len(felts))
	for i := range felts {
		felt252s[i] = &common.Felt252{Elements: felts[i]}
	}
	return felt252s
}

func getRandomResourceLimits(t *testing.T) (core.ResourceBounds, *transaction.ResourceLimits) {
	maxAmount := rand.Uint64()
	maxAmountFelt := felt.FromUint64(maxAmount)
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

func getSampleClass(t *testing.T) (felt.Felt, *core.Cairo1Class) {
	var classHash felt.Felt
	_, err := classHash.SetString("0x3cc90db763e736ca9b6c581ea4008408842b1a125947ab087438676a7e40b7b")
	require.NoError(t, err)

	client := feeder.NewTestClient(t, &utils.Sepolia)
	gw := adaptfeeder.New(client)

	class, err := gw.Class(t.Context(), &classHash)
	require.NoError(t, err)

	cairo1Class, ok := class.(*core.Cairo1Class)
	require.True(t, ok)

	return classHash, cairo1Class
}

func GetTestProposalInit(t *testing.T) (consensus.ProposalInit, *p2pconsensus.ProposalInit) {
	blockNumber := rand.Uint64()
	round := rand.Uint32()
	validRound := rand.Uint32()
	proposer, proposerBytes := getRandomFelt(t)

	consensusProposalInit := consensus.ProposalInit{
		BlockNum:   consensus.Height(blockNumber),
		Round:      consensus.Round(round),
		ValidRound: consensus.Round(validRound),
		Proposer:   proposer,
	}

	p2pProposalInit := p2pconsensus.ProposalInit{
		BlockNumber: blockNumber,
		Round:       round,
		ValidRound:  &validRound,
		Proposer:    &common.Address{Elements: proposerBytes},
	}

	return consensusProposalInit, &p2pProposalInit
}

func GetTestBlockInfo(t *testing.T) (consensus.BlockInfo, *p2pconsensus.BlockInfo) {
	blockNumber := rand.Uint64()
	timestamp := rand.Uint64()
	builder, builderBytes := getRandomFelt(t)
	l2GasPriceFri, l2GasPriceFriUint128 := getRandomUint128()
	l1GasPriceWei, l1GasPriceWeiUint128 := getRandomUint128()
	l1DataGasPriceWei, l1DataGasPriceWeiUint128 := getRandomUint128()
	ethToStrkRate, ethToStrkRateUint128 := getRandomUint128()

	consensusBlockInfo := consensus.BlockInfo{
		BlockNumber:       blockNumber,
		Builder:           builder,
		Timestamp:         timestamp,
		L2GasPriceFRI:     *l2GasPriceFri,
		L1GasPriceWEI:     *l1GasPriceWei,
		L1DataGasPriceWEI: *l1DataGasPriceWei,
		EthToStrkRate:     *ethToStrkRate,
		L1DAMode:          core.Blob,
	}

	p2pBlockInfo := p2pconsensus.BlockInfo{
		BlockNumber:       blockNumber,
		Builder:           &common.Address{Elements: builderBytes},
		Timestamp:         timestamp,
		L2GasPriceFri:     l2GasPriceFriUint128,
		L1GasPriceWei:     l1GasPriceWeiUint128,
		L1DataGasPriceWei: l1DataGasPriceWeiUint128,
		EthToStrkRate:     ethToStrkRateUint128,
		L1DaMode:          common.L1DataAvailabilityMode_Blob,
	}

	return consensusBlockInfo, &p2pBlockInfo
}

func GetTestProposalCommitment(t *testing.T) (consensus.ProposalCommitment, *p2pconsensus.ProposalCommitment) {
	blockNumber := rand.Uint64()
	timestamp := rand.Uint64()
	builder, builderBytes := getRandomFelt(t)
	parentCommitment, parentCommitmentBytes := getRandomFelt(t)
	oldStateRoot, oldStateRootBytes := getRandomFelt(t)
	versionConstantCommitment, versionConstantCommitmentBytes := getRandomFelt(t)
	stateDiffCommitment, stateDiffCommitmentBytes := getRandomFelt(t)
	transactionCommitment, transactionCommitmentBytes := getRandomFelt(t)
	eventCommitment, eventCommitmentBytes := getRandomFelt(t)
	receiptCommitment, receiptCommitmentBytes := getRandomFelt(t)
	concatenatedCounts, concatenatedCountsBytes := getRandomFelt(t)
	l1GasPriceFri, l1GasPriceFriUint128 := getRandomUint128()
	l1DataGasPriceFri, l1DataGasPriceFriUint128 := getRandomUint128()
	l2GasPriceFri, l2GasPriceFriUint128 := getRandomUint128()
	l2GasUsed, l2GasUsedUint128 := getRandomUint128()
	nextL2GasPriceFri, nextL2GasPriceFriUint128 := getRandomUint128()
	protocolVersion, protocolVersionString := semver.New(1, 2, 3, "", ""), "1.2.3"

	consensusProposalCommitment := consensus.ProposalCommitment{
		BlockNumber:               blockNumber,
		Builder:                   builder,
		ParentCommitment:          parentCommitment,
		Timestamp:                 timestamp,
		ProtocolVersion:           *protocolVersion,
		OldStateRoot:              oldStateRoot,
		VersionConstantCommitment: versionConstantCommitment,
		StateDiffCommitment:       stateDiffCommitment,
		TransactionCommitment:     transactionCommitment,
		EventCommitment:           eventCommitment,
		ReceiptCommitment:         receiptCommitment,
		ConcatenatedCounts:        concatenatedCounts,
		L1GasPriceFRI:             *l1GasPriceFri,
		L1DataGasPriceFRI:         *l1DataGasPriceFri,
		L2GasPriceFRI:             *l2GasPriceFri,
		L2GasUsed:                 *l2GasUsed,
		NextL2GasPriceFRI:         *nextL2GasPriceFri,
		L1DAMode:                  core.Blob,
	}

	p2pProposalCommitment := p2pconsensus.ProposalCommitment{
		BlockNumber:               blockNumber,
		ParentCommitment:          &common.Hash{Elements: parentCommitmentBytes},
		Builder:                   &common.Address{Elements: builderBytes},
		Timestamp:                 timestamp,
		ProtocolVersion:           protocolVersionString,
		OldStateRoot:              &common.Hash{Elements: oldStateRootBytes},
		VersionConstantCommitment: &common.Hash{Elements: versionConstantCommitmentBytes},
		StateDiffCommitment:       &common.Hash{Elements: stateDiffCommitmentBytes},
		TransactionCommitment:     &common.Hash{Elements: transactionCommitmentBytes},
		EventCommitment:           &common.Hash{Elements: eventCommitmentBytes},
		ReceiptCommitment:         &common.Hash{Elements: receiptCommitmentBytes},
		ConcatenatedCounts:        &common.Felt252{Elements: concatenatedCountsBytes},
		L1GasPriceFri:             l1GasPriceFriUint128,
		L1DataGasPriceFri:         l1DataGasPriceFriUint128,
		L2GasPriceFri:             l2GasPriceFriUint128,
		L2GasUsed:                 l2GasUsedUint128,
		NextL2GasPriceFri:         nextL2GasPriceFriUint128,
		L1DaMode:                  common.L1DataAvailabilityMode_Blob,
	}

	return consensusProposalCommitment, &p2pProposalCommitment
}

func GetTestDeclareTransaction(t *testing.T) (consensus.Transaction, *p2pconsensus.ConsensusTransaction) {
	transactionHash, transactionHashBytes := getRandomFelt(t)
	classHash, cairo1Class := getSampleClass(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	nonce, nonceBytes := getRandomFelt(t)
	version := new(core.TransactionVersion).SetUint64(3)
	compiledClassHash, compiledClassHashBytes := getRandomFelt(t)
	resourceBounds, p2pResourceBounds := getRandomResourceBounds(t)
	tip := rand.Uint64()
	paymasterData, paymasterDataBytes := getRandomFeltSlice(t)
	accountDeploymentData, accountDeploymentDataBytes := getRandomFeltSlice(t)

	consensusDeclareTransaction := consensus.Transaction{
		Transaction: &core.DeclareTransaction{
			TransactionHash:       &transactionHash,
			ClassHash:             &classHash,
			SenderAddress:         &senderAddress,
			MaxFee:                nil, // Unused field
			TransactionSignature:  transactionSignature,
			Nonce:                 &nonce,
			Version:               version,
			CompiledClassHash:     &compiledClassHash,
			ResourceBounds:        resourceBounds,
			Tip:                   tip,
			PaymasterData:         paymasterData,
			AccountDeploymentData: accountDeploymentData,
			NonceDAMode:           core.DAModeL2,
			FeeDAMode:             core.DAModeL2,
		},
		Class:       cairo1Class,
		PaidFeeOnL1: nil, // TODO: Figure out if we need this field
	}

	p2pTransaction := p2pconsensus.ConsensusTransaction{
		Txn: &p2pconsensus.ConsensusTransaction_DeclareV3{
			DeclareV3: &transaction.DeclareV3WithClass{
				Common: &transaction.DeclareV3Common{
					Sender:                    &common.Address{Elements: senderAddressBytes},
					Signature:                 &transaction.AccountSignature{Parts: toFelt252Slice(transactionSignatureBytes)},
					Nonce:                     &common.Felt252{Elements: nonceBytes},
					CompiledClassHash:         &common.Hash{Elements: compiledClassHashBytes},
					ResourceBounds:            p2pResourceBounds,
					Tip:                       tip,
					PaymasterData:             toFelt252Slice(paymasterDataBytes),
					AccountDeploymentData:     toFelt252Slice(accountDeploymentDataBytes),
					NonceDataAvailabilityMode: common.VolitionDomain_L2,
					FeeDataAvailabilityMode:   common.VolitionDomain_L2,
				},
				Class: core2p2p.AdaptCairo1Class(cairo1Class),
			},
		},
		TransactionHash: &common.Hash{Elements: transactionHashBytes},
	}

	return consensusDeclareTransaction, &p2pTransaction
}

func GetTestDeployAccountTransaction(t *testing.T) (consensus.Transaction, *p2pconsensus.ConsensusTransaction) {
	transactionHash, transactionHashBytes := getRandomFelt(t)
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

	consensusDeployAccountTransaction := consensus.Transaction{
		Transaction: &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     &transactionHash,
				ContractAddressSalt: &contractAddressSalt,
				ContractAddress:     contractAddress,
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
		},
		Class:       nil,
		PaidFeeOnL1: nil, // TODO: Figure out if we need this field
	}

	p2pTransaction := p2pconsensus.ConsensusTransaction{
		Txn: &p2pconsensus.ConsensusTransaction_DeployAccountV3{
			DeployAccountV3: &transaction.DeployAccountV3{
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
			},
		},
		TransactionHash: &common.Hash{Elements: transactionHashBytes},
	}

	return consensusDeployAccountTransaction, &p2pTransaction
}

func GetTestInvokeTransaction(t *testing.T) (consensus.Transaction, *p2pconsensus.ConsensusTransaction) {
	transactionHash, transactionHashBytes := getRandomFelt(t)
	constructorCallData, constructorCallDataBytes := getRandomFeltSlice(t)
	transactionSignature, transactionSignatureBytes := getRandomFeltSlice(t)
	version := new(core.TransactionVersion).SetUint64(3)
	nonce, nonceBytes := getRandomFelt(t)
	senderAddress, senderAddressBytes := getRandomFelt(t)
	resourceBounds, p2pResourceBounds := getRandomResourceBounds(t)
	tip := rand.Uint64()
	paymasterData, paymasterDataBytes := getRandomFeltSlice(t)

	consensusInvokeTransaction := consensus.Transaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash:       &transactionHash,
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
		},
		Class:       nil,
		PaidFeeOnL1: nil, // TODO: Figure out if we need this field
	}

	p2pTransaction := p2pconsensus.ConsensusTransaction{
		Txn: &p2pconsensus.ConsensusTransaction_InvokeV3{
			InvokeV3: &transaction.InvokeV3{
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
			},
		},
		TransactionHash: &common.Hash{Elements: transactionHashBytes},
	}

	return consensusInvokeTransaction, &p2pTransaction
}

func GetTestL1HandlerTransaction(t *testing.T) (consensus.Transaction, *p2pconsensus.ConsensusTransaction) {
	transactionHash, transactionHashBytes := getRandomFelt(t)
	contractAddress, contractAddressBytes := getRandomFelt(t)
	entryPointSelector, entryPointSelectorBytes := getRandomFelt(t)
	nonce, nonceBytes := getRandomFelt(t)
	callData, callDataBytes := getRandomFeltSlice(t)
	version := new(core.TransactionVersion).SetUint64(0)

	consensusL1HandlerTransaction := consensus.Transaction{
		Transaction: &core.L1HandlerTransaction{
			TransactionHash:    &transactionHash,
			ContractAddress:    &contractAddress,
			EntryPointSelector: &entryPointSelector,
			Nonce:              &nonce,
			CallData:           callData,
			Version:            version,
		},
		Class:       nil,
		PaidFeeOnL1: nil, // TODO: Figure out if we need this field
	}

	p2pTransaction := p2pconsensus.ConsensusTransaction{
		Txn: &p2pconsensus.ConsensusTransaction_L1Handler{
			L1Handler: &transaction.L1HandlerV0{
				Nonce:              &common.Felt252{Elements: nonceBytes},
				Address:            &common.Address{Elements: contractAddressBytes},
				EntryPointSelector: &common.Felt252{Elements: entryPointSelectorBytes},
				Calldata:           toFelt252Slice(callDataBytes),
			},
		},
		TransactionHash: &common.Hash{Elements: transactionHashBytes},
	}

	return consensusL1HandlerTransaction, &p2pTransaction
}

// StripCompilerFields strips the some fields related to compiler in the compiled class.
// It's due to the difference between the expected compiler version and the actual compiler version.
func StripCompilerFields(t *testing.T, transaction *consensus.Transaction) {
	if transaction.Class == nil {
		return
	}

	switch class := transaction.Class.(type) {
	case *core.Cairo1Class:
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

func GetTestProposalFin(t *testing.T) (consensus.ProposalFin, *p2pconsensus.ProposalFin) {
	proposalCommitment, proposalCommitmentBytes := getRandomFelt(t)
	proposalFin := consensus.ProposalFin(proposalCommitment)

	p2pProposalFin := p2pconsensus.ProposalFin{
		ProposalCommitment: &common.Hash{Elements: proposalCommitmentBytes},
	}

	return proposalFin, &p2pProposalFin
}

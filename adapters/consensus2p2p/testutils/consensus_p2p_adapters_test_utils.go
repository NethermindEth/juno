//nolint:gosec // The whole package is for testing purpose only, so it's safe to use weak random.
package testutils

import (
	"math/rand/v2"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	transactiontestutils "github.com/NethermindEth/juno/adapters/testutils"
	consensus "github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	p2pconsensus "github.com/starknet-io/starknet-p2p-specs/p2p/proto/consensus/consensus"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/transaction"
)

var TransactionBuilder = transactiontestutils.TransactionBuilder[
	consensus.Transaction,
	*p2pconsensus.ConsensusTransaction,
]{
	ToCore: func(
		transaction core.Transaction,
		class core.ClassDefinition,
		paidFeeOnL1 *felt.Felt,
	) consensus.Transaction {
		return consensus.Transaction{
			Transaction: transaction,
			Class:       class,
			PaidFeeOnL1: paidFeeOnL1,
		}
	},
	ToP2PDeclareV3: func(
		transaction *transaction.DeclareV3WithClass,
		transactionHash *common.Hash,
	) *p2pconsensus.ConsensusTransaction {
		return &p2pconsensus.ConsensusTransaction{
			Txn: &p2pconsensus.ConsensusTransaction_DeclareV3{
				DeclareV3: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PDeploy: func(
		transaction *transaction.DeployAccountV3,
		transactionHash *common.Hash,
	) *p2pconsensus.ConsensusTransaction {
		return &p2pconsensus.ConsensusTransaction{
			Txn: &p2pconsensus.ConsensusTransaction_DeployAccountV3{
				DeployAccountV3: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PInvoke: func(
		transaction *transaction.InvokeV3,
		transactionHash *common.Hash,
	) *p2pconsensus.ConsensusTransaction {
		return &p2pconsensus.ConsensusTransaction{
			Txn: &p2pconsensus.ConsensusTransaction_InvokeV3{
				InvokeV3: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PL1Handler: func(
		transaction *transaction.L1HandlerV0,
		transactionHash *common.Hash,
	) *p2pconsensus.ConsensusTransaction {
		return &p2pconsensus.ConsensusTransaction{
			Txn: &p2pconsensus.ConsensusTransaction_L1Handler{
				L1Handler: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
}

func getRandomFelt[F felt.FeltLike](t *testing.T) (F, []byte) {
	t.Helper()

	f := felt.Random[felt.Felt]()
	feltBytes := f.Bytes()
	return F(f), feltBytes[:]
}

func getRandomUint128[F felt.FeltLike]() (*F, *common.Uint128) {
	low := rand.Uint64()
	high := rand.Uint64()
	uint128 := common.Uint128{
		Low:  low,
		High: high,
	}
	value := p2p2core.AdaptUint128[F](&uint128)
	return value, &uint128
}

func GetTestProposalInit(t *testing.T) (consensus.ProposalInit, *p2pconsensus.ProposalInit) {
	blockNumber := rand.Uint64()
	round := rand.Uint32()
	validRound := rand.Uint32()
	proposer, proposerBytes := getRandomFelt[felt.Felt](t)

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
	builder, builderBytes := getRandomFelt[felt.Felt](t)
	l2GasPriceFri, l2GasPriceFriUint128 := getRandomUint128[felt.Felt]()
	l1GasPriceWei, l1GasPriceWeiUint128 := getRandomUint128[felt.Felt]()
	l1DataGasPriceWei, l1DataGasPriceWeiUint128 := getRandomUint128[felt.Felt]()
	ethToStrkRate, ethToStrkRateUint128 := getRandomUint128[felt.Felt]()

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

func GetTestProposalCommitment(t *testing.T) (
	consensus.ProposalCommitment,
	*p2pconsensus.ProposalCommitment,
) {
	blockNumber := rand.Uint64()
	timestamp := rand.Uint64()
	builder, builderBytes := getRandomFelt[felt.Felt](t)
	parentCommitment, parentCommitmentBytes := getRandomFelt[felt.Felt](t)
	oldStateRoot, oldStateRootBytes := getRandomFelt[felt.Felt](t)
	versionConstantCommitment, versionConstantCommitmentBytes := getRandomFelt[felt.Felt](t)
	stateDiffCommitment, stateDiffCommitmentBytes := getRandomFelt[felt.Felt](t)
	transactionCommitment, transactionCommitmentBytes := getRandomFelt[felt.Felt](t)
	eventCommitment, eventCommitmentBytes := getRandomFelt[felt.Felt](t)
	receiptCommitment, receiptCommitmentBytes := getRandomFelt[felt.Felt](t)
	concatenatedCounts, concatenatedCountsBytes := getRandomFelt[felt.Felt](t)
	l1GasPriceFri, l1GasPriceFriUint128 := getRandomUint128[felt.Felt]()
	l1DataGasPriceFri, l1DataGasPriceFriUint128 := getRandomUint128[felt.Felt]()
	l2GasPriceFri, l2GasPriceFriUint128 := getRandomUint128[felt.Felt]()
	l2GasUsed, l2GasUsedUint128 := getRandomUint128[felt.Felt]()
	nextL2GasPriceFri, nextL2GasPriceFriUint128 := getRandomUint128[felt.Felt]()
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

func GetTestProposalFin(t *testing.T) (consensus.ProposalFin, *p2pconsensus.ProposalFin) {
	proposalCommitment, proposalCommitmentBytes := getRandomFelt[felt.Felt](t)
	proposalFin := consensus.ProposalFin(proposalCommitment)

	p2pProposalFin := p2pconsensus.ProposalFin{
		ProposalCommitment: &common.Hash{Elements: proposalCommitmentBytes},
	}

	return proposalFin, &p2pProposalFin
}

package p2p2consensus_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/adapters/consensus2p2p"
	"github.com/NethermindEth/juno/adapters/consensus2p2p/testutils"
	"github.com/NethermindEth/juno/adapters/p2p2consensus"
	transactiontestutils "github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/require"
)

func testP2PToConsensusToP2P[C, P any](
	t *testing.T,
	getTestData func(t *testing.T) (C, *P),
	p2pToConsensus func(*P) (C, error),
	consensusToP2P func(*C) P,
) {
	consensusData, p2pData := getTestData(t)

	convertedConsensus, err := p2pToConsensus(p2pData)
	require.NoError(t, err)
	require.Equal(t, consensusData, convertedConsensus)

	convertedP2P := consensusToP2P(&convertedConsensus)
	require.Equal(t, *p2pData, convertedP2P)
}

func TestAdaptProposalInit(t *testing.T) {
	testP2PToConsensusToP2P(t, testutils.GetTestProposalInit, p2p2consensus.AdaptProposalInit, consensus2p2p.AdaptProposalInit)
}

func TestAdaptBlockInfo(t *testing.T) {
	testP2PToConsensusToP2P(t, testutils.GetTestBlockInfo, p2p2consensus.AdaptBlockInfo, consensus2p2p.AdaptBlockInfo)
}

func TestAdaptProposalCommitment(t *testing.T) {
	testP2PToConsensusToP2P(t, testutils.GetTestProposalCommitment, p2p2consensus.AdaptProposalCommitment, consensus2p2p.AdaptProposalCommitment)
}

func TestAdaptProposalTransaction(t *testing.T) {
	consensusTransactions, p2pTransactions := transactiontestutils.GetTestTransactions(
		t,
		&utils.Mainnet,
		testutils.TransactionBuilder.GetTestDeclareTransaction,
		testutils.TransactionBuilder.GetTestDeployAccountTransaction,
		testutils.TransactionBuilder.GetTestInvokeTransaction,
		testutils.TransactionBuilder.GetTestL1HandlerTransaction,
	)

	for i := range consensusTransactions {
		t.Run(fmt.Sprintf("%T", consensusTransactions[i].Transaction), func(t *testing.T) {
			convertedConsensusTransaction, err := p2p2consensus.AdaptTransaction(
				t.Context(), compiler.NewUnsafe(), p2pTransactions[i], &utils.Mainnet,
			)
			require.NoError(t, err)

			transactiontestutils.StripCompilerFields(t, consensusTransactions[i].Class)
			transactiontestutils.StripCompilerFields(t, convertedConsensusTransaction.Class)
			require.Equal(t, consensusTransactions[i], convertedConsensusTransaction)

			convertedP2PTransaction, err := consensus2p2p.AdaptTransaction(&consensusTransactions[i])
			require.NoError(t, err)
			require.Equal(t, p2pTransactions[i], convertedP2PTransaction)
		})
	}

	t.Run("Batch", func(t *testing.T) {
		transactionBatch := consensus.TransactionBatch{
			Transactions: p2pTransactions,
		}
		convertedConsensusTransactions, err := p2p2consensus.AdaptProposalTransaction(
			t.Context(), compiler.NewUnsafe(), &transactionBatch, &utils.Mainnet,
		)
		require.NoError(t, err)

		for i := range consensusTransactions {
			transactiontestutils.StripCompilerFields(t, consensusTransactions[i].Class)
			transactiontestutils.StripCompilerFields(t, convertedConsensusTransactions[i].Class)
		}
		require.Equal(t, consensusTransactions, convertedConsensusTransactions)

		convertedP2PTransactions, err := consensus2p2p.AdaptProposalTransaction(consensusTransactions)
		require.NoError(t, err)
		require.Equal(t, p2pTransactions, convertedP2PTransactions.Transactions)
	})
}

func TestAdaptProposalFin(t *testing.T) {
	testP2PToConsensusToP2P(t, testutils.GetTestProposalFin, p2p2consensus.AdaptProposalFin, consensus2p2p.AdaptProposalFin)
}

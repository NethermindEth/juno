package consensus2p2p_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/adapters/consensus2p2p"
	"github.com/NethermindEth/juno/adapters/consensus2p2p/testutils"
	"github.com/NethermindEth/juno/adapters/p2p2consensus"
	transactiontestutils "github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func testConsensusToP2PToConsensus[C, P any](
	t *testing.T,
	getTestData func(t *testing.T) (C, *P),
	consensusToP2P func(*C) P,
	p2pToConsensus func(*P) (C, error),
) {
	consensus, p2p := getTestData(t)

	convertedP2P := consensusToP2P(&consensus)
	require.Equal(t, *p2p, convertedP2P)

	convertedConsensus, err := p2pToConsensus(&convertedP2P)
	require.NoError(t, err)
	require.Equal(t, consensus, convertedConsensus)
}

func TestAdaptProposalInit(t *testing.T) {
	testConsensusToP2PToConsensus(t, testutils.GetTestProposalInit, consensus2p2p.AdaptProposalInit, p2p2consensus.AdaptProposalInit)
}

func TestAdaptBlockInfo(t *testing.T) {
	testConsensusToP2PToConsensus(t, testutils.GetTestBlockInfo, consensus2p2p.AdaptBlockInfo, p2p2consensus.AdaptBlockInfo)
}

func TestAdaptProposalCommitment(t *testing.T) {
	testConsensusToP2PToConsensus(t, testutils.GetTestProposalCommitment, consensus2p2p.AdaptProposalCommitment, p2p2consensus.AdaptProposalCommitment)
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
			convertedP2PTransaction, err := consensus2p2p.AdaptTransaction(&consensusTransactions[i])
			require.NoError(t, err)
			require.Equal(t, p2pTransactions[i], convertedP2PTransaction)

			convertedConsensusTransaction, err := p2p2consensus.AdaptTransaction(
				t.Context(), compiler.NewUnsafe(), convertedP2PTransaction, &utils.Mainnet,
			)
			require.NoError(t, err)

			transactiontestutils.StripCompilerFields(t, consensusTransactions[i].Class)
			transactiontestutils.StripCompilerFields(t, convertedConsensusTransaction.Class)
			require.Equal(t, consensusTransactions[i], convertedConsensusTransaction)
		})
	}

	t.Run("Batch", func(t *testing.T) {
		convertedP2PTransactions, err := consensus2p2p.AdaptProposalTransaction(consensusTransactions)
		require.NoError(t, err)
		require.Equal(t, p2pTransactions, convertedP2PTransactions.Transactions)

		convertedConsensusTransactions, err := p2p2consensus.AdaptProposalTransaction(
			t.Context(), compiler.NewUnsafe(), &convertedP2PTransactions, &utils.Mainnet,
		)
		require.NoError(t, err)
		for i := range consensusTransactions {
			transactiontestutils.StripCompilerFields(t, consensusTransactions[i].Class)
			transactiontestutils.StripCompilerFields(t, convertedConsensusTransactions[i].Class)
		}
		require.Equal(t, consensusTransactions, convertedConsensusTransactions)
	})
}

func TestAdaptProposalFin(t *testing.T) {
	testConsensusToP2PToConsensus(t, testutils.GetTestProposalFin, consensus2p2p.AdaptProposalFin, p2p2consensus.AdaptProposalFin)
}

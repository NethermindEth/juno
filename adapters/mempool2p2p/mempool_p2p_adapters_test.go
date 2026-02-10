package mempool2p2p_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/adapters/mempool2p2p"
	"github.com/NethermindEth/juno/adapters/mempool2p2p/testutils"
	"github.com/NethermindEth/juno/adapters/p2p2mempool"
	transactiontestutils "github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestAdaptTransaction(t *testing.T) {
	mempoolTransactions, p2pTransactions := transactiontestutils.GetTestTransactions(
		t,
		&utils.Sepolia,
		testutils.TransactionBuilder.GetTestDeclareTransaction,
		testutils.TransactionBuilder.GetTestDeployAccountTransaction,
		testutils.TransactionBuilder.GetTestInvokeTransaction,
	)

	for i := range mempoolTransactions {
		t.Run(fmt.Sprintf("%T", mempoolTransactions[i].Transaction), func(t *testing.T) {
			convertedP2PTransaction, err := mempool2p2p.AdaptTransaction(&mempoolTransactions[i])
			require.NoError(t, err)
			require.Equal(t, p2pTransactions[i], convertedP2PTransaction)

			convertedmempoolTransaction, err := p2p2mempool.AdaptTransaction(
				t.Context(), compiler.NewUnsafe(), convertedP2PTransaction, &utils.Sepolia,
			)
			require.NoError(t, err)

			transactiontestutils.StripCompilerFields(t, mempoolTransactions[i].DeclaredClass)
			transactiontestutils.StripCompilerFields(t, convertedmempoolTransaction.DeclaredClass)
			require.Equal(t, mempoolTransactions[i], convertedmempoolTransaction)
		})
	}
}

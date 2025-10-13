package p2p2core_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	transactiontestutils "github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	synctransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/transaction"
	"github.com/stretchr/testify/require"
)

var TransactionBuilder = transactiontestutils.TransactionBuilder[
	core.DeclareTransaction, *synctransaction.TransactionInBlock]{
	ToCore: func(
		transaction core.Transaction,
		class core.Class, paidFeeOnL1 *felt.Felt,
	) core.DeclareTransaction {
		return *transaction.(*core.DeclareTransaction)
	},
	ToP2PDeclareSync: func(
		transaction *synctransaction.TransactionInBlock_DeclareV0WithoutClass,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn: &synctransaction.TransactionInBlock_DeclareV0{
				DeclareV0: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
}

func TestAdaptTransactionInBlockDeclareV0(t *testing.T) {
	consensusTransactions, p2pTransactions := transactiontestutils.GetTestTransactions(
		t,
		&utils.Mainnet,
		TransactionBuilder.GetTestDeclareTransactionSync,
	)
	for i := range consensusTransactions {
		t.Run(fmt.Sprintf("%T", consensusTransactions[i].TransactionHash), func(t *testing.T) {
			convertedConsensusTransaction, err := p2p2core.AdaptTransaction(
				p2pTransactions[i],
				&utils.Mainnet,
			)
			require.NoError(t, err)
			require.Equal(t, &consensusTransactions[i], convertedConsensusTransaction)

			convertedP2PTransaction := core2p2p.AdaptTransaction(&consensusTransactions[i])
			require.Equal(t, p2pTransactions[i], convertedP2PTransaction)
		})
	}
}

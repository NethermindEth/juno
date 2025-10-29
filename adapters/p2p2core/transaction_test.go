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

var SyncTransactionBuilder = transactiontestutils.SyncTransactionBuilder[
	core.Transaction,
	*synctransaction.TransactionInBlock,
]{
	ToCore: func(
		transaction core.Transaction,
		class core.ClassDefinition,
		paidFeeOnL1 *felt.Felt,
	) core.Transaction {
		return transaction
	},
	ToP2PDeclareV0: func(
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
	ToP2PDeclareV1: func(
		transaction *synctransaction.TransactionInBlock_DeclareV1WithoutClass,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn: &synctransaction.TransactionInBlock_DeclareV1{
				DeclareV1: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PDeclareV2: func(
		transaction *synctransaction.TransactionInBlock_DeclareV2WithoutClass,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn: &synctransaction.TransactionInBlock_DeclareV2{
				DeclareV2: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PDeclareV3: func(
		transaction *synctransaction.TransactionInBlock_DeclareV3WithoutClass,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn: &synctransaction.TransactionInBlock_DeclareV3{
				DeclareV3: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PDeployV0: func(
		transaction *synctransaction.TransactionInBlock_Deploy,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn: &synctransaction.TransactionInBlock_Deploy_{
				Deploy: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PDeployV1: func(
		transaction *synctransaction.TransactionInBlock_DeployAccountV1,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn: &synctransaction.TransactionInBlock_DeployAccountV1_{
				DeployAccountV1: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PDeployV3: func(
		transaction *synctransaction.TransactionInBlock_DeployAccountV3,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn:             transaction,
			TransactionHash: transactionHash,
		}
	},
	ToP2PInvokeV0: func(
		transaction *synctransaction.TransactionInBlock_InvokeV0,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn: &synctransaction.TransactionInBlock_InvokeV0_{
				InvokeV0: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PInvokeV1: func(
		transaction *synctransaction.TransactionInBlock_InvokeV1,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn: &synctransaction.TransactionInBlock_InvokeV1_{
				InvokeV1: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PInvokeV3: func(
		transaction *synctransaction.TransactionInBlock_InvokeV3,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn:             transaction,
			TransactionHash: transactionHash,
		}
	},
	ToP2PL1Handler: func(
		transaction *synctransaction.TransactionInBlock_L1Handler,
		transactionHash *common.Hash,
	) *synctransaction.TransactionInBlock {
		return &synctransaction.TransactionInBlock{
			Txn:             transaction,
			TransactionHash: transactionHash,
		}
	},
}

func TestAdaptTransactionInBlock(t *testing.T) {
	consensusTransactions, p2pTransactions := transactiontestutils.GetTestTransactions(
		t,
		&utils.Mainnet,
		SyncTransactionBuilder.GetTestDeclareV0Transaction,
		SyncTransactionBuilder.GetTestDeclareV1Transaction,
		SyncTransactionBuilder.GetTestDeclareV2Transaction,
		SyncTransactionBuilder.GetTestDeclareV3Transaction,
		SyncTransactionBuilder.GetTestDeployTransactionV0,
		SyncTransactionBuilder.GetTestDeployAccountTransactionV1,
		SyncTransactionBuilder.GetTestDeployAccountTransactionV3,
		SyncTransactionBuilder.GetTestInvokeTransactionV0,
		SyncTransactionBuilder.GetTestInvokeTransactionV1,
		SyncTransactionBuilder.GetTestInvokeTransactionV3,
		SyncTransactionBuilder.GetTestL1HandlerTransaction,
	)
	for i := range consensusTransactions {
		t.Run(fmt.Sprintf("%T", consensusTransactions[i].Hash()), func(t *testing.T) {
			convertedConsensusTransaction, err := p2p2core.AdaptTransaction(
				p2pTransactions[i],
				&utils.Mainnet,
			)
			require.NoError(t, err)
			require.Equal(t, consensusTransactions[i], convertedConsensusTransaction)

			convertedP2PTransaction := core2p2p.AdaptTransaction(consensusTransactions[i])
			require.Equal(t, p2pTransactions[i], convertedP2PTransaction)
		})
	}
}

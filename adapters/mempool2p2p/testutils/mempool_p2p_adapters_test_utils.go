package testutils

import (
	transactiontestutils "github.com/NethermindEth/juno/adapters/testutils"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	p2pmempool "github.com/starknet-io/starknet-p2pspecs/p2p/proto/mempool/transaction"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
)

var TransactionBuilder = transactiontestutils.TransactionBuilder[
	mempool.BroadcastedTransaction,
	*p2pmempool.MempoolTransaction,
]{
	ToCore: func(
		transaction core.Transaction,
		class core.ClassDefinition,
		paidFeeOnL1 *felt.Felt,
	) mempool.BroadcastedTransaction {
		return mempool.BroadcastedTransaction{
			Transaction:   transaction,
			DeclaredClass: class,
			PaidFeeOnL1:   paidFeeOnL1,
			Proof:         nil,
		}
	},
	ToP2PDeclareV3: func(
		transaction *transaction.DeclareV3WithClass,
		transactionHash *common.Hash,
	) *p2pmempool.MempoolTransaction {
		return &p2pmempool.MempoolTransaction{
			Txn: &p2pmempool.MempoolTransaction_DeclareV3{
				DeclareV3: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PDeploy: func(
		transaction *transaction.DeployAccountV3,
		transactionHash *common.Hash,
	) *p2pmempool.MempoolTransaction {
		return &p2pmempool.MempoolTransaction{
			Txn: &p2pmempool.MempoolTransaction_DeployAccountV3{
				DeployAccountV3: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PInvoke: func(
		transaction *transaction.InvokeV3,
		transactionHash *common.Hash,
	) *p2pmempool.MempoolTransaction {
		return &p2pmempool.MempoolTransaction{
			Txn: &p2pmempool.MempoolTransaction_InvokeV3{
				InvokeV3: transaction,
			},
			TransactionHash: transactionHash,
		}
	},
	ToP2PL1Handler: nil, // Mempool doesn't support L1 handler
}

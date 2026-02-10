package p2p2consensus

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	consensus "github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	p2pconsensus "github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

func AdaptTransaction(
	ctx context.Context,
	compiler compiler.Compiler,
	t *p2pconsensus.ConsensusTransaction,
	network *utils.Network,
) (consensus.Transaction, error) {
	if err := validateConsensusTransaction(t); err != nil {
		return consensus.Transaction{}, err
	}

	var (
		tx          core.Transaction
		class       core.ClassDefinition
		paidFeeOnL1 *felt.Felt
		err         error
	)

	switch t.Txn.(type) {
	case *p2pconsensus.ConsensusTransaction_DeclareV3:
		tx, class, err = p2p2core.AdaptDeclareV3WithClass(
			ctx, compiler, t.GetDeclareV3(), t.TransactionHash,
		)
		if err != nil {
			return consensus.Transaction{}, err
		}
	case *p2pconsensus.ConsensusTransaction_DeployAccountV3:
		tx, err = p2p2core.AdaptDeployAccountV3TxnCommon(t.GetDeployAccountV3(), t.TransactionHash)
		if err != nil {
			return consensus.Transaction{}, err
		}
	case *p2pconsensus.ConsensusTransaction_InvokeV3:
		tx, err = p2p2core.AdaptInvokeV3TxnCommon(t.GetInvokeV3(), t.TransactionHash)
		if err != nil {
			return consensus.Transaction{}, err
		}
	case *p2pconsensus.ConsensusTransaction_L1Handler:
		tx = p2p2core.AdaptL1Handler(t.GetL1Handler(), t.TransactionHash)
		paidFeeOnL1 = felt.One.Clone()
	default:
		return consensus.Transaction{}, fmt.Errorf("unsupported tx type %T", t.Txn)
	}

	computedTransactionHash, err := core.TransactionHash(tx, network)
	if err != nil {
		return consensus.Transaction{}, err
	}

	if computedTransactionHash != *tx.Hash() {
		return consensus.Transaction{},
			fmt.Errorf("transaction hash mismatch: computed %s, got %s",
				&computedTransactionHash,
				tx.Hash(),
			)
	}

	return consensus.Transaction{
		Transaction: tx,
		Class:       class,
		PaidFeeOnL1: paidFeeOnL1,
	}, nil
}

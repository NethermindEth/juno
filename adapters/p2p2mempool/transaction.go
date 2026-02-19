package p2p2mempool

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	mempooltransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/mempool/transaction"
)

func validateMempoolTransaction(t *mempooltransaction.MempoolTransaction) error {
	if t == nil {
		return errors.New("mempool transaction is nil")
	}
	// TODO: consider further validation here
	if t.Txn == nil {
		return errors.New("txn must be set to a valid variant")
	}
	if t.TransactionHash == nil {
		return errors.New("transaction_hash must be set")
	}
	return nil
}

func AdaptTransaction(
	ctx context.Context,
	compiler compiler.Compiler,
	t *mempooltransaction.MempoolTransaction,
	network *utils.Network,
) (mempool.BroadcastedTransaction, error) {
	if err := validateMempoolTransaction(t); err != nil {
		return mempool.BroadcastedTransaction{}, err
	}

	var (
		tx    core.Transaction
		class core.ClassDefinition
		err   error
	)

	switch t.Txn.(type) {
	case *mempooltransaction.MempoolTransaction_DeclareV3:
		tx, class, err = p2p2core.AdaptDeclareV3WithClass(
			ctx, compiler, t.GetDeclareV3(), t.TransactionHash,
		)
		if err != nil {
			return mempool.BroadcastedTransaction{}, err
		}
	case *mempooltransaction.MempoolTransaction_DeployAccountV3:
		tx, err = p2p2core.AdaptDeployAccountV3TxnCommon(t.GetDeployAccountV3(), t.TransactionHash)
	case *mempooltransaction.MempoolTransaction_InvokeV3:
		tx, err = p2p2core.AdaptInvokeV3TxnCommon(t.GetInvokeV3(), t.TransactionHash)
	default:
		return mempool.BroadcastedTransaction{}, fmt.Errorf("unsupported tx type %T", t.Txn)
	}
	if err != nil {
		return mempool.BroadcastedTransaction{}, err
	}

	computedTransactionHash, err := core.TransactionHash(tx, network)
	if err != nil {
		return mempool.BroadcastedTransaction{}, err
	}

	if computedTransactionHash != *tx.Hash() {
		return mempool.BroadcastedTransaction{},
			fmt.Errorf("transaction hash mismatch: computed %s, got %s",
				&computedTransactionHash,
				tx.Hash(),
			)
	}

	return mempool.BroadcastedTransaction{
		Transaction:   tx,
		DeclaredClass: class,
		PaidFeeOnL1:   nil,
		Proof:         "",
	}, nil
}

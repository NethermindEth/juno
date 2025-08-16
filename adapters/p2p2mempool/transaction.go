package p2p2mempool

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/mempool"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
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

func AdaptTransaction(t *mempooltransaction.MempoolTransaction) (mempool.BroadcastedTransaction, error) {
	if t == nil {
		return mempool.BroadcastedTransaction{}, errors.New("transaction is nil")
	}
	if err := validateMempoolTransaction(t); err != nil {
		return mempool.BroadcastedTransaction{}, err
	}

	switch t.Txn.(type) {
	case *mempooltransaction.MempoolTransaction_DeclareV3:
		tx := t.GetDeclareV3()
		// Todo: we pass in CompiledClassHash here, but in sync we pass in ClassHash.
		// Are we expected to compile the class hash here???
		class := p2p2core.AdaptCairo1Class(tx.Class)
		classHash, err := class.Hash()
		classHashBytes := classHash.Bytes()
		if err != nil {
			return mempool.BroadcastedTransaction{}, err
		}
		return mempool.BroadcastedTransaction{
			Transaction:   p2p2core.AdaptDeclareV3TxnCommon(tx.Common, &common.Hash{Elements: classHashBytes[:]}, t.TransactionHash),
			DeclaredClass: &class,
			PaidFeeOnL1:   nil, // TODO: Double check if we need this field
		}, nil

	case *mempooltransaction.MempoolTransaction_DeployAccountV3:
		tx := t.GetDeployAccountV3()
		return mempool.BroadcastedTransaction{
			Transaction:   p2p2core.AdaptDeployAccountV3TxnCommon(tx, t.TransactionHash),
			DeclaredClass: nil,
			PaidFeeOnL1:   nil, // TODO: Double check if we need this field
		}, nil

	case *mempooltransaction.MempoolTransaction_InvokeV3:
		tx := t.GetInvokeV3()
		return mempool.BroadcastedTransaction{
			Transaction:   p2p2core.AdaptInvokeV3TxnCommon(tx, t.TransactionHash),
			DeclaredClass: nil,
			PaidFeeOnL1:   nil, // TODO: Double check if we need this field
		}, nil

	default:
		return mempool.BroadcastedTransaction{}, fmt.Errorf("unsupported tx type %T", t.Txn)
	}
}

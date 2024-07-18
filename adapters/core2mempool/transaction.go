package core2mempool

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/mempool"
)

func AdaptTransaction(txn *core.Transaction) (*mempool.BroadcastedTransaction, error) {

	// switch txn.(type) {
	// case core.DeclareTransaction:

	// case core.InvokeTransaction, core.DeployAccountTransaction, core.L1HandlerTransaction:

	// default:
	// 	return nil, fmt.Errorf("unknown transaction type")
	// }
	// panic("Need to implement AdaptTransaction ")
	return &mempool.BroadcastedTransaction{}, nil
}

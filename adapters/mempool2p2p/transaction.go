package mempool2p2p

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	mempooltransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/mempool/transaction"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
)

func toHash(felt *felt.Felt) *common.Hash {
	feltBytes := felt.Bytes()
	return &common.Hash{Elements: feltBytes[:]}
}

func AdaptTransaction(tx *mempool.BroadcastedTransaction) (*mempooltransaction.MempoolTransaction, error) {
	if tx.Transaction == nil {
		return nil, errors.New("transaction is nil")
	}

	switch t := tx.Transaction.(type) {
	case *core.DeclareTransaction:
		if tx.DeclaredClass == nil {
			return nil, errors.New("declared class is nil")
		}

		switch class := tx.DeclaredClass.(type) {
		case *core.SierraClass:
			return &mempooltransaction.MempoolTransaction{
				Txn: &mempooltransaction.MempoolTransaction_DeclareV3{
					DeclareV3: &transaction.DeclareV3WithClass{
						Common: core2p2p.AdaptDeclareV3Common(t),
						Class:  core2p2p.AdaptSierraClass(class),
					},
				},
				TransactionHash: toHash(t.TransactionHash),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported class type %T", class)
		}

	case *core.DeployAccountTransaction:
		return &mempooltransaction.MempoolTransaction{
			Txn: &mempooltransaction.MempoolTransaction_DeployAccountV3{
				DeployAccountV3: core2p2p.AdaptDeployAccountV3Transaction(t),
			},
			TransactionHash: toHash(t.TransactionHash),
		}, nil
	case *core.InvokeTransaction:
		return &mempooltransaction.MempoolTransaction{
			Txn: &mempooltransaction.MempoolTransaction_InvokeV3{
				InvokeV3: core2p2p.AdaptInvokeV3Transaction(t),
			},
			TransactionHash: toHash(t.TransactionHash),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported tx type %T", t)
	}
}

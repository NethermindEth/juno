package consensus2p2p

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	consensus "github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core"
	p2pconsensus "github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
)

func AdaptTransaction(tx *consensus.Transaction) (*p2pconsensus.ConsensusTransaction, error) {
	if tx.Transaction == nil {
		return nil, errors.New("transaction is nil")
	}

	switch t := tx.Transaction.(type) {
	case *core.DeclareTransaction:
		if tx.Class == nil {
			return nil, errors.New("class is nil")
		}

		switch class := tx.Class.(type) {
		case *core.SierraClass:
			return &p2pconsensus.ConsensusTransaction{
				Txn: &p2pconsensus.ConsensusTransaction_DeclareV3{
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
		return &p2pconsensus.ConsensusTransaction{
			Txn: &p2pconsensus.ConsensusTransaction_DeployAccountV3{
				DeployAccountV3: core2p2p.AdaptDeployAccountV3Transaction(t),
			},
			TransactionHash: toHash(t.TransactionHash),
		}, nil
	case *core.InvokeTransaction:
		return &p2pconsensus.ConsensusTransaction{
			Txn: &p2pconsensus.ConsensusTransaction_InvokeV3{
				InvokeV3: core2p2p.AdaptInvokeV3Transaction(t),
			},
			TransactionHash: toHash(t.TransactionHash),
		}, nil
	case *core.L1HandlerTransaction:
		return &p2pconsensus.ConsensusTransaction{
			Txn: &p2pconsensus.ConsensusTransaction_L1Handler{
				L1Handler: core2p2p.AdaptL1HandlerTransaction(t),
			},
			TransactionHash: toHash(t.TransactionHash),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported tx type %T", t)
	}
}

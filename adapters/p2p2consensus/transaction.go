package p2p2consensus

import (
	"fmt"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/core"
	p2pconsensus "github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

func AdaptTransaction(msg *p2pconsensus.ConsensusTransaction) (core.Transaction, core.Class, error) {
	if msg == nil {
		return nil, nil, nil
	}
	if err := validateConsensusTransaction(msg); err != nil {
		return nil, nil, err
	}
	switch msg.Txn.(type) {
	case *p2pconsensus.ConsensusTransaction_DeclareV3:
		tx := msg.GetDeclareV3()
		// Todo: we pass in CompiledClassHash here, but in sync we pass in ClassHash.
		// Are we expected to compile the class hash here???
		class := p2p2core.AdaptCairo1Class(tx.Class)
		return p2p2core.AdaptDeclareV3TxnCommon(tx.Common, tx.Common.CompiledClassHash, msg.TransactionHash), &class, nil

	case *p2pconsensus.ConsensusTransaction_DeployAccountV3:
		tx := msg.GetDeployAccountV3()
		return p2p2core.AdaptDeployAccountV3TxnCommon(tx, msg.TransactionHash), nil, nil
	case *p2pconsensus.ConsensusTransaction_InvokeV3:
		tx := msg.GetInvokeV3()
		return p2p2core.AdaptInvokeV3TxnCommon(tx, msg.TransactionHash), nil, nil
	case *p2pconsensus.ConsensusTransaction_L1Handler:
		tx := msg.GetL1Handler()
		return p2p2core.AdaptL1Handler(tx, msg.TransactionHash), nil, nil
	default:
		panic(fmt.Errorf("unsupported tx type %T", msg.Txn))
	}
}

package p2p2consensus

import (
	"fmt"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/core"
	p2pconsensus "github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

func AdaptTransaction(t *p2pconsensus.ConsensusTransaction) (core.Transaction, core.Class, error) {
	if t == nil {
		return nil, nil, nil
	}
	if err := validateConsensusTransaction(t); err != nil {
		return nil, nil, err
	}
	switch t.Txn.(type) {
	case *p2pconsensus.ConsensusTransaction_DeclareV3:
		tx := t.GetDeclareV3()
		// Todo: we pass in CompiledClassHash here, but in sync we pass in ClassHash.
		// Are we expected to compile the class hash here???
		class := p2p2core.AdaptCairo1Class(tx.Class)
		return p2p2core.AdaptDeclareV3TxnCommon(tx.Common, tx.Common.CompiledClassHash, t.TransactionHash), &class, nil
	case *p2pconsensus.ConsensusTransaction_DeployAccountV3:
		tx := t.GetDeployAccountV3()
		return p2p2core.AdaptDeployAccountV3TxnCommon(tx, t.TransactionHash), nil, nil
	case *p2pconsensus.ConsensusTransaction_InvokeV3:
		tx := t.GetInvokeV3()
		return p2p2core.AdaptInvokeV3TxnCommon(tx, t.TransactionHash), nil, nil
	case *p2pconsensus.ConsensusTransaction_L1Handler:
		tx := t.GetL1Handler()
		return p2p2core.AdaptL1Handler(tx, t.TransactionHash), nil, nil
	default:
		panic(fmt.Errorf("unsupported tx type %T", t.Txn))
	}
}

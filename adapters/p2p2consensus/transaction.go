package p2p2consensus

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	consensus "github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	p2pconsensus "github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
)

func AdaptTransaction(t *p2pconsensus.ConsensusTransaction) (consensus.Transaction, error) {
	if t == nil {
		return consensus.Transaction{}, errors.New("transaction is nil")
	}
	if err := validateConsensusTransaction(t); err != nil {
		return consensus.Transaction{}, err
	}

	switch t.Txn.(type) {
	case *p2pconsensus.ConsensusTransaction_DeclareV3:
		tx := t.GetDeclareV3()
		// Todo: we pass in CompiledClassHash here, but in sync we pass in ClassHash.
		// Are we expected to compile the class hash here???
		class := p2p2core.AdaptCairo1Class(tx.Class)
		classHash, err := class.Hash()
		classHashBytes := classHash.Bytes()
		if err != nil {
			return consensus.Transaction{}, err
		}
		return consensus.Transaction{
			Transaction: p2p2core.AdaptDeclareV3TxnCommon(tx.Common, &common.Hash{Elements: classHashBytes[:]}, t.TransactionHash),
			Class:       &class,
			PaidFeeOnL1: nil,
		}, nil

	case *p2pconsensus.ConsensusTransaction_DeployAccountV3:
		tx := t.GetDeployAccountV3()
		return consensus.Transaction{
			Transaction: p2p2core.AdaptDeployAccountV3TxnCommon(tx, t.TransactionHash),
			Class:       nil,
			PaidFeeOnL1: nil,
		}, nil

	case *p2pconsensus.ConsensusTransaction_InvokeV3:
		tx := t.GetInvokeV3()
		return consensus.Transaction{
			Transaction: p2p2core.AdaptInvokeV3TxnCommon(tx, t.TransactionHash),
			Class:       nil,
			PaidFeeOnL1: nil,
		}, nil

	case *p2pconsensus.ConsensusTransaction_L1Handler:
		tx := t.GetL1Handler()
		return consensus.Transaction{
			Transaction: p2p2core.AdaptL1Handler(tx, t.TransactionHash),
			Class:       nil,
			PaidFeeOnL1: felt.One.Clone(),
		}, nil

	default:
		return consensus.Transaction{}, fmt.Errorf("unsupported tx type %T", t.Txn)
	}
}

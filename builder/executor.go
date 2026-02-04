package builder

import (
	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"go.uber.org/zap"
)

type Executor interface {
	RunTxns(*BuildState, []mempool.BroadcastedTransaction) (err error)
	Finish(*BuildState) (blockchain.SimulateResult, error)
}

type executor struct {
	log          utils.Logger
	blockchain   *blockchain.Blockchain
	vm           vm.VM
	disableFees  bool
	skipValidate bool
}

func NewExecutor(
	blockchain *blockchain.Blockchain,
	vm vm.VM,
	log utils.Logger,
	disableFees bool,
	skipValidate bool,
) Executor {
	return &executor{
		blockchain:   blockchain,
		vm:           vm,
		log:          log,
		disableFees:  disableFees,
		skipValidate: skipValidate,
	}
}

// RunTxns executes the provided transaction and applies the state changes
// to the preconfirmed state
func (e *executor) RunTxns(state *BuildState, txns []mempool.BroadcastedTransaction) (err error) {
	if len(txns) == 0 {
		return nil
	}

	headState, headCloser, err := e.blockchain.HeadState()
	if err != nil {
		return err
	}
	defer func() {
		if err := headCloser(); err != nil {
			e.log.Error("failed to close head state", zap.Error(err))
		}
	}()

	// Create a state writer for the transaction execution
	stateWriter := core.NewPendingStateWriter(
		state.Preconfirmed.StateUpdate.StateDiff,
		state.Preconfirmed.NewClasses,
		headState,
	)

	// Prepare declared classes, if any
	var declaredClasses []core.ClassDefinition
	paidFeesOnL1 := []*felt.Felt{}
	coreTxns := make([]core.Transaction, len(txns))
	for i, txn := range txns {
		if txn.DeclaredClass != nil {
			declaredClasses = append(declaredClasses, txn.DeclaredClass)
		}
		if txn.PaidFeeOnL1 != nil {
			paidFeesOnL1 = append(paidFeesOnL1, txn.PaidFeeOnL1)
		}
		coreTxns[i] = txn.Transaction
	}

	// Execute the transaction
	vmResults, err := e.vm.Execute(
		coreTxns,
		declaredClasses,
		paidFeesOnL1,
		&vm.BlockInfo{
			Header:                state.Preconfirmed.Block.Header,
			BlockHashToBeRevealed: state.RevealedBlockHash,
		},
		stateWriter,
		e.disableFees,
		e.skipValidate,
		false,
		true,
		false,
		false,
	)
	if err != nil {
		return err
	}

	// Handle declared classes for declare transactions
	for i, trace := range vmResults.Traces {
		if trace.StateDiff.DeclaredClasses != nil ||
			trace.StateDiff.DeprecatedDeclaredClasses != nil {
			if err := e.processClassDeclaration(&txns[i], &stateWriter); err != nil {
				return err
			}
		}
	}

	// Adapt results to core type (which use reference types)
	receipts := make([]*core.TransactionReceipt, len(txns))
	stateDiffs := make([]*core.StateDiff, len(vmResults.Traces))
	for i, trace := range vmResults.Traces {
		adaptedStateDiff := vm2core.AdaptStateDiff(trace.StateDiff)
		adaptedReceipt := vm2core.Receipt(vmResults.OverallFees[i], txns[i].Transaction, &vmResults.Traces[i], &vmResults.Receipts[i])
		receipts[i] = &adaptedReceipt
		stateDiffs[i] = &adaptedStateDiff
	}

	// Update preconfirmed block with transaction results
	updatePreconfirmedBlock(state.Preconfirmed, receipts, coreTxns, stateDiffs)

	for i := range vmResults.GasConsumed {
		state.L2GasConsumed += vmResults.GasConsumed[i].L2Gas
	}

	return nil
}

// processClassDeclaration handles class declaration storage for declare transactions
func (e *executor) processClassDeclaration(
	txn *mempool.BroadcastedTransaction,
	state *core.PendingStateWriter,
) error {
	if t, ok := (txn.Transaction).(*core.DeclareTransaction); ok {
		if err := state.SetContractClass(t.ClassHash, txn.DeclaredClass); err != nil {
			e.log.Error("failed to set contract class", zap.Error(err))
			return err
		}

		if t.CompiledClassHash != nil {
			if err := state.SetCompiledClassHash(t.ClassHash, t.CompiledClassHash); err != nil {
				e.log.Error("failed to SetCompiledClassHash", zap.Error(err))
				return err
			}
		}
	}
	return nil
}

// updatePreconfirmedBlock updates the preconfirmed block with transaction results
func updatePreconfirmedBlock(
	preconfirmed *core.PreConfirmed,
	receipts []*core.TransactionReceipt,
	transactions []core.Transaction,
	stateDiffs []*core.StateDiff,
) {
	preconfirmed.Block.Receipts = append(preconfirmed.Block.Receipts, receipts...)
	preconfirmed.TransactionStateDiffs = append(preconfirmed.TransactionStateDiffs, stateDiffs...)
	preconfirmed.Block.Transactions = append(preconfirmed.Block.Transactions, transactions...)
	preconfirmed.Block.TransactionCount += uint64(len(transactions))
	for _, receipt := range receipts {
		preconfirmed.Block.EventCount += uint64(len(receipt.Events))
	}
	for _, stateDiff := range stateDiffs {
		preconfirmed.StateUpdate.StateDiff.Merge(stateDiff)
	}
}

func (e *executor) Finish(state *BuildState) (blockchain.SimulateResult, error) {
	return e.blockchain.Simulate(state.Preconfirmed.Block, state.Preconfirmed.StateUpdate, state.Preconfirmed.NewClasses, nil)
}

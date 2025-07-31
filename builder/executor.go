package builder

import (
	"github.com/NethermindEth/juno/adapters/vm2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
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
// to the pending state
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
			e.log.Errorw("failed to close head state", "err", err)
		}
	}()

	// Create a state writer for the transaction execution
	stateWriter := sync.NewPendingStateWriter(state.Pending.StateUpdate.StateDiff, state.Pending.NewClasses, headState)

	// Prepare declared classes, if any
	var declaredClasses []core.Class
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
			Header:                state.Pending.Block.Header,
			BlockHashToBeRevealed: state.RevealedBlockHash,
		},
		stateWriter,
		e.blockchain.Network(),
		e.disableFees, e.skipValidate, false, true, false)
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
	mergedStateDiff := vm2core.AdaptStateDiff(vmResults.Traces[0].StateDiff)
	for i, trace := range vmResults.Traces {
		adaptedStateDiff := vm2core.AdaptStateDiff(trace.StateDiff)
		mergedStateDiff.Merge(&adaptedStateDiff)
		adaptedReceipt := vm2core.Receipt(vmResults.OverallFees[i], txns[i].Transaction, &vmResults.Traces[i], &vmResults.Receipts[i])
		receipts[i] = &adaptedReceipt
	}

	// Update pending block with transaction results
	updatePendingBlock(state.Pending, receipts, coreTxns, mergedStateDiff)

	for i := range vmResults.GasConsumed {
		state.L2GasConsumed += vmResults.GasConsumed[i].L2Gas
	}

	return nil
}

// processClassDeclaration handles class declaration storage for declare transactions
func (e *executor) processClassDeclaration(txn *mempool.BroadcastedTransaction, state *sync.PendingStateWriter) error {
	if t, ok := (txn.Transaction).(*core.DeclareTransaction); ok {
		if err := state.SetContractClass(t.ClassHash, txn.DeclaredClass); err != nil {
			e.log.Errorw("failed to set contract class", "err", err)
			return err
		}

		if t.CompiledClassHash != nil {
			if err := state.SetCompiledClassHash(t.ClassHash, t.CompiledClassHash); err != nil {
				e.log.Errorw("failed to SetCompiledClassHash", "err", err)
				return err
			}
		}
	}
	return nil
}

// updatePendingBlock updates the pending block with transaction results
func updatePendingBlock(
	pending *sync.Pending,
	receipts []*core.TransactionReceipt,
	transactions []core.Transaction,
	stateDiff core.StateDiff,
) {
	pending.Block.Receipts = append(pending.Block.Receipts, receipts...)
	pending.Block.Transactions = append(pending.Block.Transactions, transactions...)
	pending.Block.TransactionCount += uint64(len(transactions))
	for _, receipt := range receipts {
		pending.Block.EventCount += uint64(len(receipt.Events))
	}
	pending.StateUpdate.StateDiff.Merge(&stateDiff)
}

func (e *executor) Finish(state *BuildState) (blockchain.SimulateResult, error) {
	return e.blockchain.Simulate(state.Pending.Block, state.Pending.StateUpdate, state.Pending.NewClasses, nil)
}

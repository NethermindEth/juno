package rpcv10

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

/****************************************************
		VM Adapters
*****************************************************/

func AdaptVMTransactionTrace(trace *vm.TransactionTrace) TransactionTrace {
	var validateInvocation *rpcv9.FunctionInvocation
	if trace.ValidateInvocation != nil && trace.Type != vm.TxnL1Handler {
		validateInvocation = utils.HeapPtr(rpcv9.AdaptVMFunctionInvocation(trace.ValidateInvocation))
	}

	var feeTransferInvocation *rpcv9.FunctionInvocation
	if trace.FeeTransferInvocation != nil && trace.Type != vm.TxnL1Handler {
		feeTransferInvocation = utils.HeapPtr(
			rpcv9.AdaptVMFunctionInvocation(trace.FeeTransferInvocation),
		)
	}

	var constructorInvocation *rpcv9.FunctionInvocation
	var executeInvocation *rpcv9.ExecuteInvocation
	var functionInvocation *rpcv9.ExecuteInvocation

	switch trace.Type {
	case vm.TxnDeployAccount, vm.TxnDeploy:
		if trace.ConstructorInvocation != nil {
			constructorInvocation = utils.HeapPtr(
				rpcv9.AdaptVMFunctionInvocation(trace.ConstructorInvocation),
			)
		}
	case vm.TxnInvoke:
		if trace.ExecuteInvocation != nil {
			executeInvocation = utils.HeapPtr(rpcv9.AdaptVMExecuteInvocation(trace.ExecuteInvocation))
		}
	case vm.TxnL1Handler:
		if trace.FunctionInvocation != nil {
			functionInvocation = utils.HeapPtr(rpcv9.AdaptVMExecuteInvocation(trace.FunctionInvocation))
		}
	}

	var resources *rpcv9.ExecutionResources
	if trace.ExecutionResources != nil {
		resources = utils.HeapPtr(rpcv9.AdaptVMExecutionResources(trace.ExecutionResources))
	}

	var stateDiff *StateDiff
	if trace.StateDiff != nil {
		stateDiff = utils.HeapPtr(AdaptVMStateDiff(trace.StateDiff))
	}

	return TransactionTrace{
		Type:                  rpcv9.TransactionType(trace.Type),
		ValidateInvocation:    validateInvocation,
		ExecuteInvocation:     executeInvocation,
		FeeTransferInvocation: feeTransferInvocation,
		ConstructorInvocation: constructorInvocation,
		FunctionInvocation:    functionInvocation,
		StateDiff:             stateDiff,
		ExecutionResources:    resources,
	}
}

func AdaptVMStateDiff(vmStateDiff *vm.StateDiff) StateDiff {
	// Adapt storage diffs
	adaptedStorageDiffs := make([]rpcv6.StorageDiff, len(vmStateDiff.StorageDiffs))
	for index := range vmStateDiff.StorageDiffs {
		vmStorageDiff := &vmStateDiff.StorageDiffs[index]

		// Adapt storage entries
		adaptedEntries := make([]rpcv6.Entry, len(vmStorageDiff.StorageEntries))
		for entryIndex := range vmStorageDiff.StorageEntries {
			vmEntry := &vmStorageDiff.StorageEntries[entryIndex]

			adaptedEntries[entryIndex] = rpcv6.Entry{
				Key:   vmEntry.Key,
				Value: vmEntry.Value,
			}
		}

		adaptedStorageDiffs[index] = rpcv6.StorageDiff{
			Address:        vmStorageDiff.Address,
			StorageEntries: adaptedEntries,
		}
	}

	// Adapt nonces
	adaptedNonces := make([]rpcv6.Nonce, len(vmStateDiff.Nonces))
	for index := range vmStateDiff.Nonces {
		vmNonce := &vmStateDiff.Nonces[index]

		adaptedNonces[index] = rpcv6.Nonce{
			ContractAddress: vmNonce.ContractAddress,
			Nonce:           vmNonce.Nonce,
		}
	}

	// Adapt deployed contracts
	adaptedDeployedContracts := make([]rpcv6.DeployedContract, len(vmStateDiff.DeployedContracts))
	for index := range vmStateDiff.DeployedContracts {
		vmDeployedContract := &vmStateDiff.DeployedContracts[index]

		adaptedDeployedContracts[index] = rpcv6.DeployedContract{
			Address:   vmDeployedContract.Address,
			ClassHash: vmDeployedContract.ClassHash,
		}
	}

	// Adapt declared classes
	adaptedDeclaredClasses := make([]rpcv6.DeclaredClass, len(vmStateDiff.DeclaredClasses))
	for index := range vmStateDiff.DeclaredClasses {
		vmDeclaredClass := &vmStateDiff.DeclaredClasses[index]

		adaptedDeclaredClasses[index] = rpcv6.DeclaredClass{
			ClassHash:         vmDeclaredClass.ClassHash,
			CompiledClassHash: vmDeclaredClass.CompiledClassHash,
		}
	}

	// Adapt replaced classes
	adaptedReplacedClasses := make([]rpcv6.ReplacedClass, len(vmStateDiff.ReplacedClasses))
	for index := range vmStateDiff.ReplacedClasses {
		vmReplacedClass := &vmStateDiff.ReplacedClasses[index]

		adaptedReplacedClasses[index] = rpcv6.ReplacedClass{
			ContractAddress: vmReplacedClass.ContractAddress,
			ClassHash:       vmReplacedClass.ClassHash,
		}
	}

	// Adapt migrated classes
	adaptedMigratedClasses := make([]MigratedCompiledClass, len(vmStateDiff.MigratedCompiledClasses))
	for index := range vmStateDiff.MigratedCompiledClasses {
		vmMigratedClass := &vmStateDiff.MigratedCompiledClasses[index]

		adaptedMigratedClasses[index] = MigratedCompiledClass{
			ClassHash:         vmMigratedClass.ClassHash,
			CompiledClassHash: vmMigratedClass.CompiledClassHash,
		}
	}

	return StateDiff{
		StorageDiffs:              adaptedStorageDiffs,
		Nonces:                    adaptedNonces,
		DeployedContracts:         adaptedDeployedContracts,
		DeprecatedDeclaredClasses: vmStateDiff.DeprecatedDeclaredClasses,
		DeclaredClasses:           adaptedDeclaredClasses,
		ReplacedClasses:           adaptedReplacedClasses,
		MigratedCompiledClasses:   adaptedMigratedClasses,
	}
}

/****************************************************
		Feeder Adapters
*****************************************************/

func AdaptFeederBlockTrace(
	block *core.Block,
	blockTrace *starknet.BlockTrace,
) ([]TracedBlockTransaction, error) {
	if blockTrace == nil {
		return nil, nil
	}

	if len(block.Transactions) != len(blockTrace.Traces) {
		return nil, errors.New("mismatched number of txs and traces")
	}

	// Adapt every feeder block trace to rpc v9 trace
	adaptedTraces := make([]TracedBlockTransaction, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]

		trace := TransactionTrace{
			Type: getTransactionType(block.Transactions[index]),
		}

		if feederTrace.FeeTransferInvocation != nil && trace.Type != rpcv9.TxnL1Handler {
			trace.FeeTransferInvocation = utils.HeapPtr(
				rpcv9.AdaptFeederFunctionInvocation(feederTrace.FeeTransferInvocation),
			)
		}

		if feederTrace.ValidateInvocation != nil && trace.Type != rpcv9.TxnL1Handler {
			trace.ValidateInvocation = utils.HeapPtr(
				rpcv9.AdaptFeederFunctionInvocation(feederTrace.ValidateInvocation),
			)
		}

		var fnInvocation *rpcv9.FunctionInvocation
		if fct := feederTrace.FunctionInvocation; fct != nil {
			fnInvocation = utils.HeapPtr(rpcv9.AdaptFeederFunctionInvocation(fct))
		}

		switch trace.Type {
		case rpcv9.TxnDeploy, rpcv9.TxnDeployAccount:
			trace.ConstructorInvocation = fnInvocation
		case rpcv9.TxnInvoke:
			trace.ExecuteInvocation = new(rpcv9.ExecuteInvocation)
			if feederTrace.RevertError != "" {
				trace.ExecuteInvocation.RevertReason = feederTrace.RevertError
			} else {
				trace.ExecuteInvocation.FunctionInvocation = fnInvocation
			}
		case rpcv9.TxnL1Handler:
			trace.FunctionInvocation = new(rpcv9.ExecuteInvocation)
			if feederTrace.RevertError != "" {
				trace.FunctionInvocation.RevertReason = feederTrace.RevertError
			} else {
				trace.FunctionInvocation.FunctionInvocation = fnInvocation
			}
		}

		adaptedTraces[index] = TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		}
	}

	return adaptedTraces, nil
}

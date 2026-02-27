package rpcv10

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

/****************************************************
		VM Trace Adapters
*****************************************************/

func AdaptVMTransactionTrace(trace *vm.TransactionTrace) TransactionTrace {
	var validateInvocation *FunctionInvocation
	if trace.ValidateInvocation != nil && trace.Type != vm.TxnL1Handler {
		validateInvocation = utils.HeapPtr(AdaptVMFunctionInvocation(trace.ValidateInvocation))
	}

	var feeTransferInvocation *FunctionInvocation
	if trace.FeeTransferInvocation != nil && trace.Type != vm.TxnL1Handler {
		feeTransferInvocation = utils.HeapPtr(
			AdaptVMFunctionInvocation(trace.FeeTransferInvocation),
		)
	}

	var constructorInvocation *FunctionInvocation
	var executeInvocation *ExecuteInvocation
	var functionInvocation *ExecuteInvocation

	switch trace.Type {
	case vm.TxnDeployAccount, vm.TxnDeploy:
		if trace.ConstructorInvocation != nil {
			constructorInvocation = utils.HeapPtr(
				AdaptVMFunctionInvocation(trace.ConstructorInvocation),
			)
		}
	case vm.TxnInvoke:
		if trace.ExecuteInvocation != nil {
			executeInvocation = utils.HeapPtr(AdaptVMExecuteInvocation(trace.ExecuteInvocation))
		}
	case vm.TxnL1Handler:
		if trace.FunctionInvocation != nil {
			functionInvocation = utils.HeapPtr(AdaptVMExecuteInvocation(trace.FunctionInvocation))
		}
	}

	var resources *ExecutionResources
	if trace.ExecutionResources != nil {
		resources = utils.HeapPtr(AdaptVMExecutionResources(trace.ExecutionResources))
	}

	var stateDiff *StateDiff
	if trace.StateDiff != nil {
		stateDiff = utils.HeapPtr(AdaptVMStateDiff(trace.StateDiff))
	}

	traceType := TransactionType(trace.Type)
	if traceType == TxnDeploy {
		// There is no DEPLOY_TXN_TRACE thus we need to convert the type to `DEPLOY_ACCOUNT`
		//nolint:lll // url exceeds line limit
		// see https://github.com/starkware-libs/starknet-specs/blob/b57ccf8fcbc32f89a42f088ff9c8b93115bfd40a/api/starknet_trace_api_openrpc.json#L235
		traceType = TxnDeployAccount
	}

	return TransactionTrace{
		Type:                  traceType,
		ValidateInvocation:    validateInvocation,
		ExecuteInvocation:     executeInvocation,
		FeeTransferInvocation: feeTransferInvocation,
		ConstructorInvocation: constructorInvocation,
		FunctionInvocation:    functionInvocation,
		StateDiff:             stateDiff,
		ExecutionResources:    resources,
	}
}

func AdaptVMExecuteInvocation(vmFnInvocation *vm.ExecuteInvocation) ExecuteInvocation {
	var functionInvocation *FunctionInvocation
	if vmFnInvocation.FunctionInvocation != nil {
		functionInvocation = utils.HeapPtr(AdaptVMFunctionInvocation(vmFnInvocation.FunctionInvocation))
	}

	return ExecuteInvocation{
		RevertReason:       vmFnInvocation.RevertReason,
		FunctionInvocation: functionInvocation,
	}
}

func AdaptVMFunctionInvocation(vmFnInvocation *vm.FunctionInvocation) FunctionInvocation {
	// Adapt inner calls
	adaptedCalls := make([]FunctionInvocation, len(vmFnInvocation.Calls))
	for index := range vmFnInvocation.Calls {
		adaptedCalls[index] = AdaptVMFunctionInvocation(&vmFnInvocation.Calls[index])
	}

	// Adapt events
	adaptedEvents := make([]OrderedEvent, len(vmFnInvocation.Events))
	for index := range vmFnInvocation.Events {
		vmEvent := &vmFnInvocation.Events[index]

		adaptedEvents[index] = OrderedEvent{
			Order: vmEvent.Order,
			Keys:  vmEvent.Keys,
			Data:  vmEvent.Data,
		}
	}

	// Adapt messages
	adaptedMessages := make([]OrderedL2toL1Message, len(vmFnInvocation.Messages))
	for index := range vmFnInvocation.Messages {
		vmMessage := &vmFnInvocation.Messages[index]

		adaptedMessages[index] = OrderedL2toL1Message{
			Order: vmMessage.Order,
			From:  vmMessage.From,
			// todo(rdr): we shouldn't need to do this casting but it affects a lot of code to
			//            do it right
			To:      (*felt.Felt)(vmMessage.To),
			Payload: vmMessage.Payload,
		}
	}

	// Adapt execution resources
	var adaptedResources *InnerExecutionResources
	if r := vmFnInvocation.ExecutionResources; r != nil {
		adaptedResources = &InnerExecutionResources{
			L1Gas: r.L1Gas,
			L2Gas: r.L2Gas,
		}
	}

	return FunctionInvocation{
		ContractAddress:    vmFnInvocation.ContractAddress,
		EntryPointSelector: vmFnInvocation.EntryPointSelector,
		Calldata:           vmFnInvocation.Calldata,
		CallerAddress:      vmFnInvocation.CallerAddress,
		ClassHash:          vmFnInvocation.ClassHash,
		EntryPointType:     vmFnInvocation.EntryPointType,
		CallType:           vmFnInvocation.CallType,
		Result:             vmFnInvocation.Result,
		Calls:              adaptedCalls,
		Events:             adaptedEvents,
		Messages:           adaptedMessages,
		ExecutionResources: adaptedResources,
		IsReverted:         vmFnInvocation.IsReverted,
	}
}

func AdaptVMExecutionResources(r *vm.ExecutionResources) ExecutionResources {
	return ExecutionResources{
		InnerExecutionResources: InnerExecutionResources{
			L1Gas: r.L1Gas,
			L2Gas: r.L2Gas,
		},
		L1DataGas: r.L1DataGas,
	}
}

func AdaptVMStateDiff(vmStateDiff *vm.StateDiff) StateDiff {
	// Adapt storage diffs
	adaptedStorageDiffs := make([]StorageDiff, len(vmStateDiff.StorageDiffs))
	for index := range vmStateDiff.StorageDiffs {
		vmStorageDiff := &vmStateDiff.StorageDiffs[index]

		// Adapt storage entries
		adaptedEntries := make([]Entry, len(vmStorageDiff.StorageEntries))
		for entryIndex := range vmStorageDiff.StorageEntries {
			vmEntry := &vmStorageDiff.StorageEntries[entryIndex]

			adaptedEntries[entryIndex] = Entry{
				Key:   vmEntry.Key,
				Value: vmEntry.Value,
			}
		}

		adaptedStorageDiffs[index] = StorageDiff{
			Address:        vmStorageDiff.Address,
			StorageEntries: adaptedEntries,
		}
	}

	// Adapt nonces
	adaptedNonces := make([]Nonce, len(vmStateDiff.Nonces))
	for index := range vmStateDiff.Nonces {
		vmNonce := &vmStateDiff.Nonces[index]

		adaptedNonces[index] = Nonce{
			ContractAddress: vmNonce.ContractAddress,
			Nonce:           vmNonce.Nonce,
		}
	}

	// Adapt deployed contracts
	adaptedDeployedContracts := make([]DeployedContract, len(vmStateDiff.DeployedContracts))
	for index := range vmStateDiff.DeployedContracts {
		vmDeployedContract := &vmStateDiff.DeployedContracts[index]

		adaptedDeployedContracts[index] = DeployedContract{
			Address:   vmDeployedContract.Address,
			ClassHash: vmDeployedContract.ClassHash,
		}
	}

	// Adapt declared classes
	adaptedDeclaredClasses := make([]DeclaredClassDiff, len(vmStateDiff.DeclaredClasses))
	for index := range vmStateDiff.DeclaredClasses {
		vmDeclaredClass := &vmStateDiff.DeclaredClasses[index]

		adaptedDeclaredClasses[index] = DeclaredClassDiff{
			ClassHash:         vmDeclaredClass.ClassHash,
			CompiledClassHash: vmDeclaredClass.CompiledClassHash,
		}
	}

	// Adapt replaced classes
	adaptedReplacedClasses := make([]ReplacedClass, len(vmStateDiff.ReplacedClasses))
	for index := range vmStateDiff.ReplacedClasses {
		vmReplacedClass := &vmStateDiff.ReplacedClasses[index]

		adaptedReplacedClasses[index] = ReplacedClass{
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

func adaptVMInitialReads(vmInitialReads *vm.InitialReads) InitialReads {
	if vmInitialReads == nil {
		return InitialReads{
			Storage:           []StorageEntry{},
			Nonces:            []NonceEntry{},
			ClassHashes:       []ClassHashEntry{},
			DeclaredContracts: []DeclaredContractEntry{},
		}
	}

	storage := make([]StorageEntry, len(vmInitialReads.Storage))
	for i, s := range vmInitialReads.Storage {
		storage[i] = StorageEntry{
			ContractAddress: s.ContractAddress,
			Key:             s.Key,
			Value:           s.Value,
		}
	}

	nonces := make([]NonceEntry, len(vmInitialReads.Nonces))
	for i, n := range vmInitialReads.Nonces {
		nonces[i] = NonceEntry{
			ContractAddress: n.ContractAddress,
			Nonce:           n.Nonce,
		}
	}

	classHashes := make([]ClassHashEntry, len(vmInitialReads.ClassHashes))
	for i, ch := range vmInitialReads.ClassHashes {
		classHashes[i] = ClassHashEntry{
			ContractAddress: ch.ContractAddress,
			ClassHash:       ch.ClassHash,
		}
	}

	declaredContracts := make([]DeclaredContractEntry, len(vmInitialReads.DeclaredContracts))
	for i, dc := range vmInitialReads.DeclaredContracts {
		declaredContracts[i] = DeclaredContractEntry{
			ClassHash:  dc.ClassHash,
			IsDeclared: dc.IsDeclared,
		}
	}

	return InitialReads{
		Storage:           storage,
		Nonces:            nonces,
		ClassHashes:       classHashes,
		DeclaredContracts: declaredContracts,
	}
}

/****************************************************
		Feeder Trace Adapters
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

		if feederTrace.FeeTransferInvocation != nil && trace.Type != TxnL1Handler {
			trace.FeeTransferInvocation = utils.HeapPtr(
				AdaptFeederFunctionInvocation(feederTrace.FeeTransferInvocation),
			)
		}

		if feederTrace.ValidateInvocation != nil && trace.Type != TxnL1Handler {
			trace.ValidateInvocation = utils.HeapPtr(
				AdaptFeederFunctionInvocation(feederTrace.ValidateInvocation),
			)
		}

		var fnInvocation *FunctionInvocation
		if fct := feederTrace.FunctionInvocation; fct != nil {
			fnInvocation = utils.HeapPtr(AdaptFeederFunctionInvocation(fct))
		}

		switch trace.Type {
		case TxnDeploy, TxnDeployAccount:
			trace.ConstructorInvocation = fnInvocation
		case TxnInvoke:
			trace.ExecuteInvocation = new(ExecuteInvocation)
			if feederTrace.RevertError != "" {
				trace.ExecuteInvocation.RevertReason = feederTrace.RevertError
			} else {
				trace.ExecuteInvocation.FunctionInvocation = fnInvocation
			}
		case TxnL1Handler:
			trace.FunctionInvocation = new(ExecuteInvocation)
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

func AdaptFeederFunctionInvocation(snFnInvocation *starknet.FunctionInvocation) FunctionInvocation {
	// Adapt inner calls
	adaptedCalls := make([]FunctionInvocation, len(snFnInvocation.InternalCalls))
	for index := range snFnInvocation.InternalCalls {
		adaptedCalls[index] = AdaptFeederFunctionInvocation(&snFnInvocation.InternalCalls[index])
	}

	// Adapt events
	adaptedEvents := make([]OrderedEvent, len(snFnInvocation.Events))
	for index := range snFnInvocation.Events {
		snEvent := &snFnInvocation.Events[index]

		adaptedEvents[index] = OrderedEvent{
			Order: snEvent.Order,
			Keys:  utils.Map(snEvent.Keys, utils.HeapPtr[felt.Felt]),
			Data:  utils.Map(snEvent.Data, utils.HeapPtr[felt.Felt]),
		}
	}

	// Adapt messages
	adaptedMessages := make([]OrderedL2toL1Message, len(snFnInvocation.Messages))
	for index := range snFnInvocation.Messages {
		snMessage := &snFnInvocation.Messages[index]

		toAddr, _ := felt.FromString[felt.Felt](snMessage.ToAddr)

		adaptedMessages[index] = OrderedL2toL1Message{
			Order:   snMessage.Order,
			From:    &snFnInvocation.ContractAddress,
			To:      &toAddr,
			Payload: utils.Map(snMessage.Payload, utils.HeapPtr[felt.Felt]),
		}
	}

	return FunctionInvocation{
		ContractAddress:    snFnInvocation.ContractAddress,
		EntryPointSelector: snFnInvocation.Selector,
		Calldata:           snFnInvocation.Calldata,
		CallerAddress:      snFnInvocation.CallerAddress,
		ClassHash:          snFnInvocation.ClassHash,
		EntryPointType:     snFnInvocation.EntryPointType,
		CallType:           snFnInvocation.CallType,
		Result:             snFnInvocation.Result,
		Calls:              adaptedCalls,
		Events:             adaptedEvents,
		Messages:           adaptedMessages,
		ExecutionResources: utils.HeapPtr(
			adaptFeederExecutionResources(&snFnInvocation.ExecutionResources)),
		IsReverted: snFnInvocation.Failed,
	}
}

func adaptFeederExecutionResources(resources *starknet.ExecutionResources) InnerExecutionResources {
	var l1Gas, l2Gas uint64
	if tgs := resources.TotalGasConsumed; tgs != nil {
		l1Gas = tgs.L1Gas
		l2Gas = tgs.L2Gas
	}

	return InnerExecutionResources{
		L1Gas: l1Gas,
		L2Gas: l2Gas,
	}
}

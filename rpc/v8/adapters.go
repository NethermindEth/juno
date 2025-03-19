package rpcv8

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	hintRunnerZero "github.com/NethermindEth/cairo-vm-go/pkg/hintrunner/zero"
	"github.com/NethermindEth/cairo-vm-go/pkg/parsers/zero"
	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/jinzhu/copier"
)

/****************************************************
		VM Adapters
*****************************************************/

func AdaptVMTransactionTrace(trace *vm.TransactionTrace) TransactionTrace {
	var validateInvocation *FunctionInvocation
	if trace.ValidateInvocation != nil && trace.Type != vm.TxnL1Handler {
		validateInvocation = utils.HeapPtr(adaptVMFunctionInvocation(trace.ValidateInvocation))
	}

	var feeTransferInvocation *FunctionInvocation
	if trace.FeeTransferInvocation != nil && trace.Type != vm.TxnL1Handler {
		feeTransferInvocation = utils.HeapPtr(adaptVMFunctionInvocation(trace.FeeTransferInvocation))
	}

	var constructorInvocation *FunctionInvocation
	var executeInvocation *ExecuteInvocation
	var functionInvocation *FunctionInvocation

	switch trace.Type {
	case vm.TxnDeployAccount, vm.TxnDeploy:
		if trace.ConstructorInvocation != nil {
			constructorInvocation = utils.HeapPtr(adaptVMFunctionInvocation(trace.ConstructorInvocation))
		}
	case vm.TxnInvoke:
		if trace.ExecuteInvocation != nil {
			executeInvocation = utils.HeapPtr(adaptVMExecuteInvocation(trace.ExecuteInvocation))
		}
	case vm.TxnL1Handler:
		if trace.FunctionInvocation != nil {
			functionInvocation = utils.HeapPtr(adaptVMFunctionInvocation(trace.FunctionInvocation))
		}
	}

	var resources *ExecutionResources
	if trace.ExecutionResources != nil {
		resources = utils.HeapPtr(adaptVMExecutionResources(trace.ExecutionResources))
	}

	var stateDiff *rpcv6.StateDiff
	if trace.StateDiff != nil {
		stateDiff = utils.HeapPtr(rpcv6.AdaptVMStateDiff(trace.StateDiff))
	}

	return TransactionTrace{
		Type:                  TransactionType(trace.Type),
		ValidateInvocation:    validateInvocation,
		ExecuteInvocation:     executeInvocation,
		FeeTransferInvocation: feeTransferInvocation,
		ConstructorInvocation: constructorInvocation,
		FunctionInvocation:    functionInvocation,
		StateDiff:             stateDiff,
		ExecutionResources:    resources,
	}
}

func adaptVMExecuteInvocation(vmFnInvocation *vm.ExecuteInvocation) ExecuteInvocation {
	var functionInvocation *FunctionInvocation
	if vmFnInvocation.FunctionInvocation != nil {
		functionInvocation = utils.HeapPtr(adaptVMFunctionInvocation(vmFnInvocation.FunctionInvocation))
	}

	return ExecuteInvocation{
		RevertReason:       vmFnInvocation.RevertReason,
		FunctionInvocation: functionInvocation,
	}
}

func adaptVMFunctionInvocation(vmFnInvocation *vm.FunctionInvocation) FunctionInvocation {
	// Adapt inner calls
	adaptedCalls := make([]FunctionInvocation, len(vmFnInvocation.Calls))
	for index := range vmFnInvocation.Calls {
		adaptedCalls[index] = adaptVMFunctionInvocation(&vmFnInvocation.Calls[index])
	}

	// Adapt events
	adaptedEvents := make([]rpcv6.OrderedEvent, len(vmFnInvocation.Events))
	for index := range vmFnInvocation.Events {
		vmEvent := &vmFnInvocation.Events[index]

		adaptedEvents[index] = rpcv6.OrderedEvent{
			Order: vmEvent.Order,
			Keys:  vmEvent.Keys,
			Data:  vmEvent.Data,
		}
	}

	// Adapt messages
	adaptedMessages := make([]rpcv6.OrderedL2toL1Message, len(vmFnInvocation.Messages))
	for index := range vmFnInvocation.Messages {
		vmMessage := &vmFnInvocation.Messages[index]

		toAddr, _ := new(felt.Felt).SetString(vmMessage.To)

		adaptedMessages[index] = rpcv6.OrderedL2toL1Message{
			Order:   vmMessage.Order,
			From:    vmMessage.From,
			To:      toAddr,
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

func adaptVMExecutionResources(r *vm.ExecutionResources) ExecutionResources {
	return ExecutionResources{
		InnerExecutionResources: InnerExecutionResources{
			L1Gas: r.L1Gas,
			L2Gas: r.L2Gas,
		},
		L1DataGas: r.L1DataGas,
	}
}

/****************************************************
		Feeder Adapters
*****************************************************/

func AdaptFeederBlockTrace(block *BlockWithTxs, blockTrace *starknet.BlockTrace) ([]TracedBlockTransaction, error) {
	if blockTrace == nil {
		return nil, nil
	}

	if len(block.Transactions) != len(blockTrace.Traces) {
		return nil, errors.New("mismatched number of txs and traces")
	}

	// Adapt every feeder block trace to rpc v8 trace
	adaptedTraces := make([]TracedBlockTransaction, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]

		trace := TransactionTrace{
			Type: block.Transactions[index].Type,
		}

		if feederTrace.FeeTransferInvocation != nil && trace.Type != TxnL1Handler {
			trace.FeeTransferInvocation = utils.HeapPtr(adaptFeederFunctionInvocation(feederTrace.FeeTransferInvocation))
		}

		if feederTrace.ValidateInvocation != nil && trace.Type != TxnL1Handler {
			trace.ValidateInvocation = utils.HeapPtr(adaptFeederFunctionInvocation(feederTrace.ValidateInvocation))
		}

		var fnInvocation *FunctionInvocation
		if fct := feederTrace.FunctionInvocation; fct != nil {
			fnInvocation = utils.HeapPtr(adaptFeederFunctionInvocation(fct))
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
			trace.FunctionInvocation = fnInvocation
		}

		adaptedTraces[index] = TracedBlockTransaction{
			TransactionHash: &feederTrace.TransactionHash,
			TraceRoot:       &trace,
		}
	}

	return adaptedTraces, nil
}

func adaptFeederFunctionInvocation(snFnInvocation *starknet.FunctionInvocation) FunctionInvocation {
	// Adapt inner calls
	adaptedCalls := make([]FunctionInvocation, len(snFnInvocation.InternalCalls))
	for index := range snFnInvocation.InternalCalls {
		adaptedCalls[index] = adaptFeederFunctionInvocation(&snFnInvocation.InternalCalls[index])
	}

	// Adapt events
	adaptedEvents := make([]rpcv6.OrderedEvent, len(snFnInvocation.Events))
	for index := range snFnInvocation.Events {
		snEvent := &snFnInvocation.Events[index]

		adaptedEvents[index] = rpcv6.OrderedEvent{
			Order: snEvent.Order,
			Keys:  utils.Map(snEvent.Keys, utils.HeapPtr[felt.Felt]),
			Data:  utils.Map(snEvent.Data, utils.HeapPtr[felt.Felt]),
		}
	}

	// Adapt messages
	adaptedMessages := make([]rpcv6.OrderedL2toL1Message, len(snFnInvocation.Messages))
	for index := range snFnInvocation.Messages {
		snMessage := &snFnInvocation.Messages[index]

		toAddr, _ := new(felt.Felt).SetString(snMessage.ToAddr)

		adaptedMessages[index] = rpcv6.OrderedL2toL1Message{
			Order:   snMessage.Order,
			From:    &snFnInvocation.ContractAddress,
			To:      toAddr,
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
		ExecutionResources: utils.HeapPtr(adaptFeederExecutionResources(&snFnInvocation.ExecutionResources)),
		IsReverted:         snFnInvocation.Failed,
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

func adaptTxToFeeder(rpcTx *Transaction) starknet.Transaction {
	var adaptedResourceBounds *map[starknet.Resource]starknet.ResourceBounds
	if rpcTx.ResourceBounds != nil {
		adaptedResourceBounds = utils.HeapPtr(adaptResourceBoundsToFeeder(*rpcTx.ResourceBounds))
	}

	var adaptedNonceDAMode *starknet.DataAvailabilityMode
	if rpcTx.NonceDAMode != nil {
		adaptedNonceDAMode = utils.HeapPtr(starknet.DataAvailabilityMode(*rpcTx.NonceDAMode))
	}

	var adaptedFeeDAMode *starknet.DataAvailabilityMode
	if rpcTx.FeeDAMode != nil {
		adaptedFeeDAMode = utils.HeapPtr(starknet.DataAvailabilityMode(*rpcTx.FeeDAMode))
	}

	return starknet.Transaction{
		Hash:                  rpcTx.Hash,
		Version:               rpcTx.Version,
		ContractAddress:       rpcTx.ContractAddress,
		ContractAddressSalt:   rpcTx.ContractAddressSalt,
		ClassHash:             rpcTx.ClassHash,
		ConstructorCallData:   rpcTx.ConstructorCallData,
		Type:                  starknet.TransactionType(rpcTx.Type),
		SenderAddress:         rpcTx.SenderAddress,
		MaxFee:                rpcTx.MaxFee,
		Signature:             rpcTx.Signature,
		CallData:              rpcTx.CallData,
		EntryPointSelector:    rpcTx.EntryPointSelector,
		Nonce:                 rpcTx.Nonce,
		CompiledClassHash:     rpcTx.CompiledClassHash,
		ResourceBounds:        adaptedResourceBounds,
		Tip:                   rpcTx.Tip,
		NonceDAMode:           adaptedNonceDAMode,
		FeeDAMode:             adaptedFeeDAMode,
		AccountDeploymentData: rpcTx.AccountDeploymentData,
		PaymasterData:         rpcTx.PaymasterData,
	}
}

func adaptResourceBoundsToFeeder(rb map[Resource]ResourceBounds) map[starknet.Resource]starknet.ResourceBounds {
	feederResourceBounds := make(map[starknet.Resource]starknet.ResourceBounds)
	for resource, bounds := range rb {
		feederResourceBounds[starknet.Resource(resource)] = starknet.ResourceBounds{
			MaxAmount:       bounds.MaxAmount,
			MaxPricePerUnit: bounds.MaxPricePerUnit,
		}
	}

	return feederResourceBounds
}

func adaptFeederTransactionStatus(txStatus *starknet.TransactionStatus) (*TransactionStatus, error) {
	var status TransactionStatus

	switch finalityStatus := txStatus.FinalityStatus; finalityStatus {
	case starknet.AcceptedOnL1:
		status.Finality = TxnStatusAcceptedOnL1
	case starknet.AcceptedOnL2:
		status.Finality = TxnStatusAcceptedOnL2
	case starknet.Received:
		status.Finality = TxnStatusReceived
	case starknet.NotReceived:
		return nil, errTransactionNotFound
	default:
		return nil, fmt.Errorf("unknown finality status: %v", finalityStatus)
	}

	switch txStatus.ExecutionStatus {
	case starknet.Succeeded:
		status.Execution = TxnSuccess
	case starknet.Reverted:
		status.Execution = TxnFailure
		status.FailureReason = txStatus.RevertError
	case starknet.Rejected:
		status.Finality = TxnStatusRejected
		status.FailureReason = txStatus.RevertError
	default: // Omit the field on error. It's optional in the spec.
	}

	return &status, nil
}

/****************************************************
		Core Adapters
*****************************************************/

func adaptCoreBlockHeader(header *core.Header) BlockHeader {
	var blockNumber *uint64
	// if header.Hash == nil it's a pending block
	if header.Hash != nil {
		blockNumber = &header.Number
	}

	sequencerAddress := header.SequencerAddress
	if sequencerAddress == nil {
		sequencerAddress = &felt.Zero
	}

	var l1DAMode rpcv6.L1DAMode
	switch header.L1DAMode {
	case core.Blob:
		l1DAMode = rpcv6.Blob
	case core.Calldata:
		l1DAMode = rpcv6.Calldata
	}

	var l1DataGasPrice rpcv6.ResourcePrice
	if header.L1DataGasPrice != nil {
		l1DataGasPrice = rpcv6.ResourcePrice{
			InWei: utils.HeapPtr(nilToZero(header.L1DataGasPrice.PriceInWei)),
			InFri: utils.HeapPtr(nilToZero(header.L1DataGasPrice.PriceInFri)),
		}
	} else {
		l1DataGasPrice = rpcv6.ResourcePrice{
			InWei: &felt.Zero,
			InFri: &felt.Zero,
		}
	}

	var l2GasPrice rpcv6.ResourcePrice
	if header.L2GasPrice != nil {
		l2GasPrice = rpcv6.ResourcePrice{
			InWei: utils.HeapPtr(nilToZero(header.L2GasPrice.PriceInWei)),
			InFri: utils.HeapPtr(nilToZero(header.L2GasPrice.PriceInFri)),
		}
	} else {
		l2GasPrice = rpcv6.ResourcePrice{
			InWei: &felt.Zero,
			InFri: &felt.Zero,
		}
	}

	return BlockHeader{
		BlockHeader: rpcv6.BlockHeader{
			Hash:             header.Hash,
			ParentHash:       header.ParentHash,
			Number:           blockNumber,
			NewRoot:          header.GlobalStateRoot,
			Timestamp:        header.Timestamp,
			SequencerAddress: sequencerAddress,
			L1GasPrice: &rpcv6.ResourcePrice{
				InWei: header.L1GasPriceETH,
				InFri: utils.HeapPtr(nilToZero(header.L1GasPriceSTRK)),
			},
			L1DataGasPrice:  &l1DataGasPrice,
			L1DAMode:        &l1DAMode,
			StarknetVersion: header.ProtocolVersion,
		},
		L2GasPrice: &l2GasPrice,
	}
}

func nilToZero(f *felt.Felt) felt.Felt {
	if f == nil {
		return felt.Zero
	}

	return *f
}

func adaptCoreExecutionResources(resources *core.ExecutionResources) ExecutionResources {
	var l1Gas, l2Gas, l1DataGas uint64
	if tgc := resources.TotalGasConsumed; tgc != nil {
		l1Gas = tgc.L1Gas
		l2Gas = tgc.L2Gas
		l1DataGas = tgc.L1DataGas
	}

	return ExecutionResources{
		InnerExecutionResources: InnerExecutionResources{
			L1Gas: l1Gas,
			L2Gas: l2Gas,
		},
		L1DataGas: l1DataGas,
	}
}

func adaptDeclaredClassToCore(declaredClass json.RawMessage) (core.Class, error) {
	var feederClass starknet.ClassDefinition
	err := json.Unmarshal(declaredClass, &feederClass)
	if err != nil {
		return nil, err
	}

	switch {
	case feederClass.V1 != nil:
		compiledClass, cErr := compiler.Compile(feederClass.V1)
		if cErr != nil {
			return nil, cErr
		}
		return sn2core.AdaptCairo1Class(feederClass.V1, compiledClass)
	case feederClass.V0 != nil:
		program := feederClass.V0.Program

		// strip the quotes
		if len(program) < 2 {
			return nil, errors.New("invalid program")
		}
		base64Program := string(program[1 : len(program)-1])

		feederClass.V0.Program, err = utils.Gzip64Decode(base64Program)
		if err != nil {
			return nil, err
		}

		return sn2core.AdaptCairo0Class(feederClass.V0)
	default:
		return nil, errors.New("empty class")
	}
}

func adaptCoreCairo0Class(class *core.Cairo0Class) (*CasmCompiledContractClass, error) {
	program, err := utils.Gzip64Decode(class.Program)
	if err != nil {
		return nil, err
	}

	var cairo0 zero.ZeroProgram
	err = json.Unmarshal(program, &cairo0)
	if err != nil {
		return nil, err
	}

	bytecode := make([]*felt.Felt, len(cairo0.Data))
	for i, str := range cairo0.Data {
		f, err := new(felt.Felt).SetString(str)
		if err != nil {
			return nil, err
		}
		bytecode[i] = f
	}

	classHints, err := hintRunnerZero.GetZeroHints(&cairo0)
	if err != nil {
		return nil, err
	}

	//nolint:prealloc
	var hints [][2]any // slice of 2-element tuples where first value is pc, and second value is slice of hints
	for pc, hintItems := range utils.SortedMap(classHints) {
		hints = append(hints, [2]any{pc, hintItems})
	}
	rawHints, err := json.Marshal(hints)
	if err != nil {
		return nil, err
	}

	adaptEntryPoint := func(ep core.EntryPoint) CasmEntryPoint {
		return CasmEntryPoint{
			Offset:   ep.Offset,
			Selector: ep.Selector,
			Builtins: nil,
		}
	}

	result := &CasmCompiledContractClass{
		EntryPointsByType: EntryPointsByType{
			Constructor: utils.Map(class.Constructors, adaptEntryPoint),
			External:    utils.Map(class.Externals, adaptEntryPoint),
			L1Handler:   utils.Map(class.L1Handlers, adaptEntryPoint),
		},
		Prime:                  cairo0.Prime,
		Bytecode:               bytecode,
		CompilerVersion:        cairo0.CompilerVersion,
		Hints:                  json.RawMessage(rawHints),
		BytecodeSegmentLengths: nil, // Cairo 0 classes don't have this field (it was introduced since Sierra 1.5.0)
	}

	return result, nil
}

func adaptCoreCompiledClass(class *core.CompiledClass) CasmCompiledContractClass {
	adaptEntryPoint := func(ep core.CompiledEntryPoint) CasmEntryPoint {
		return CasmEntryPoint{
			Offset:   new(felt.Felt).SetUint64(ep.Offset),
			Selector: ep.Selector,
			Builtins: ep.Builtins,
		}
	}

	result := CasmCompiledContractClass{
		EntryPointsByType: EntryPointsByType{
			Constructor: utils.Map(class.Constructor, adaptEntryPoint),
			External:    utils.Map(class.External, adaptEntryPoint),
			L1Handler:   utils.Map(class.L1Handler, adaptEntryPoint),
		},
		Prime:                  utils.ToHex(class.Prime),
		CompilerVersion:        class.CompilerVersion,
		Bytecode:               class.Bytecode,
		Hints:                  class.Hints,
		BytecodeSegmentLengths: collectSegmentLengths(class.BytecodeSegmentLengths),
	}

	return result
}

func collectSegmentLengths(segmentLengths core.SegmentLengths) []int {
	if len(segmentLengths.Children) == 0 {
		return []int{int(segmentLengths.Length)}
	}

	var result []int
	for _, child := range segmentLengths.Children {
		result = append(result, collectSegmentLengths(child)...)
	}

	return result
}

func adaptBroadcastedTransactionToCore(broadcastedTxn *BroadcastedTransaction,
	network *utils.Network,
) (core.Transaction, core.Class, *felt.Felt, error) {
	var feederTxn starknet.Transaction
	if err := copier.Copy(&feederTxn, broadcastedTxn.Transaction); err != nil {
		return nil, nil, nil, err
	}

	txn, err := sn2core.AdaptTransaction(&feederTxn)
	if err != nil {
		return nil, nil, nil, err
	}

	var declaredClass core.Class
	if len(broadcastedTxn.ContractClass) != 0 {
		declaredClass, err = adaptDeclaredClassToCore(broadcastedTxn.ContractClass)
		if err != nil {
			return nil, nil, nil, err
		}
	} else if broadcastedTxn.Type == TxnDeclare {
		return nil, nil, nil, errors.New("declare without a class definition")
	}

	if t, ok := txn.(*core.DeclareTransaction); ok {
		t.ClassHash, err = declaredClass.Hash()
		if err != nil {
			return nil, nil, nil, err
		}
	}

	txnHash, err := core.TransactionHash(txn, network)
	if err != nil {
		return nil, nil, nil, err
	}

	var paidFeeOnL1 *felt.Felt
	switch t := txn.(type) {
	case *core.DeclareTransaction:
		t.TransactionHash = txnHash
	case *core.InvokeTransaction:
		t.TransactionHash = txnHash
	case *core.DeployAccountTransaction:
		t.TransactionHash = txnHash
	case *core.L1HandlerTransaction:
		t.TransactionHash = txnHash
		paidFeeOnL1 = broadcastedTxn.PaidFeeOnL1
	default:
		return nil, nil, nil, errors.New("unsupported transaction")
	}

	if txn.Hash() == nil {
		return nil, nil, nil, errors.New("deprecated transaction type")
	}
	return txn, declaredClass, paidFeeOnL1, nil
}

func AdaptCoreTransaction(t core.Transaction) Transaction {
	var txn Transaction

	switch v := t.(type) {
	case *core.DeployTransaction:
		// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1521
		txn = Transaction{
			Type:                TxnDeploy,
			Hash:                v.Hash(),
			ClassHash:           v.ClassHash,
			Version:             v.Version.AsFelt(),
			ContractAddressSalt: v.ContractAddressSalt,
			ConstructorCallData: &v.ConstructorCallData,
		}
	case *core.InvokeTransaction:
		txn = adaptCoreInvokeTransaction(v)
	case *core.DeclareTransaction:
		txn = adaptCoreDeclareTransaction(v)
	case *core.DeployAccountTransaction:
		txn = adaptCoreDeployAccountTrandaction(v)
	case *core.L1HandlerTransaction:
		nonce := v.Nonce
		if nonce == nil {
			nonce = &felt.Zero
		}
		txn = Transaction{
			Type:               TxnL1Handler,
			Hash:               v.Hash(),
			Version:            v.Version.AsFelt(),
			Nonce:              nonce,
			ContractAddress:    v.ContractAddress,
			EntryPointSelector: v.EntryPointSelector,
			CallData:           &v.CallData,
		}
	default:
		panic("not a transaction")
	}

	if txn.Version.IsZero() && txn.Type != TxnL1Handler {
		txn.Nonce = nil
	}

	return txn
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1605
func adaptCoreInvokeTransaction(t *core.InvokeTransaction) Transaction {
	tx := Transaction{
		Type:               TxnInvoke,
		Hash:               t.Hash(),
		MaxFee:             t.MaxFee,
		Version:            t.Version.AsFelt(),
		Signature:          utils.HeapPtr(t.Signature()),
		Nonce:              t.Nonce,
		CallData:           &t.CallData,
		ContractAddress:    t.ContractAddress,
		SenderAddress:      t.SenderAddress,
		EntryPointSelector: t.EntryPointSelector,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = utils.HeapPtr(adaptCoreResourceBounds(t.ResourceBounds))
		tx.Tip = new(felt.Felt).SetUint64(t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.AccountDeploymentData = &t.AccountDeploymentData
		tx.NonceDAMode = utils.HeapPtr(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = utils.HeapPtr(DataAvailabilityMode(t.FeeDAMode))
	}

	return tx
}

// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L1340
func adaptCoreDeclareTransaction(t *core.DeclareTransaction) Transaction {
	tx := Transaction{
		Hash:              t.Hash(),
		Type:              TxnDeclare,
		MaxFee:            t.MaxFee,
		Version:           t.Version.AsFelt(),
		Signature:         utils.HeapPtr(t.Signature()),
		Nonce:             t.Nonce,
		ClassHash:         t.ClassHash,
		SenderAddress:     t.SenderAddress,
		CompiledClassHash: t.CompiledClassHash,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = utils.HeapPtr(adaptCoreResourceBounds(t.ResourceBounds))
		tx.Tip = new(felt.Felt).SetUint64(t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.AccountDeploymentData = &t.AccountDeploymentData
		tx.NonceDAMode = utils.HeapPtr(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = utils.HeapPtr(DataAvailabilityMode(t.FeeDAMode))
	}

	return tx
}

func adaptCoreDeployAccountTrandaction(t *core.DeployAccountTransaction) Transaction {
	tx := Transaction{
		Hash:                t.Hash(),
		MaxFee:              t.MaxFee,
		Version:             t.Version.AsFelt(),
		Signature:           utils.HeapPtr(t.Signature()),
		Nonce:               t.Nonce,
		Type:                TxnDeployAccount,
		ContractAddressSalt: t.ContractAddressSalt,
		ConstructorCallData: &t.ConstructorCallData,
		ClassHash:           t.ClassHash,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = utils.HeapPtr(adaptCoreResourceBounds(t.ResourceBounds))
		tx.Tip = new(felt.Felt).SetUint64(t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.NonceDAMode = utils.HeapPtr(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = utils.HeapPtr(DataAvailabilityMode(t.FeeDAMode))
	}

	return tx
}

func adaptCoreResourceBounds(rb map[core.Resource]core.ResourceBounds) map[Resource]ResourceBounds {
	rpcResourceBounds := make(map[Resource]ResourceBounds)

	for resource, bounds := range rb {
		rpcResourceBounds[Resource(resource)] = ResourceBounds{
			MaxAmount:       new(felt.Felt).SetUint64(bounds.MaxAmount),
			MaxPricePerUnit: bounds.MaxPricePerUnit,
		}
	}

	return rpcResourceBounds
}

// todo(Kirill): try to replace core.Transaction with rpc.Transaction type
func AdaptCoreReceipt(receipt *core.TransactionReceipt, txn core.Transaction, finalityStatus TxnFinalityStatus,
	blockHash *felt.Felt, blockNumber uint64,
) TransactionReceipt {
	messages := make([]*MsgToL1, len(receipt.L2ToL1Message))
	for idx, msg := range receipt.L2ToL1Message {
		messages[idx] = &MsgToL1{
			To:      msg.To,
			Payload: msg.Payload,
			From:    msg.From,
		}
	}

	events := make([]*Event, len(receipt.Events))
	for idx, event := range receipt.Events {
		events[idx] = &Event{
			From: event.From,
			Keys: event.Keys,
			Data: event.Data,
		}
	}

	var messageHash string
	var contractAddress *felt.Felt
	switch v := txn.(type) {
	case *core.DeployTransaction:
		contractAddress = v.ContractAddress
	case *core.DeployAccountTransaction:
		contractAddress = v.ContractAddress
	case *core.L1HandlerTransaction:
		messageHash = "0x" + hex.EncodeToString(v.MessageHash())
	}

	var receiptBlockNumber *uint64
	// case for pending blocks: they don't have blockHash and therefore no block number
	if blockHash != nil {
		receiptBlockNumber = &blockNumber
	}

	var es TxnExecutionStatus
	if receipt.Reverted {
		es = TxnFailure
	} else {
		es = TxnSuccess
	}

	var executionResources *ExecutionResources
	if receipt.ExecutionResources != nil {
		executionResources = utils.HeapPtr(adaptCoreExecutionResources(receipt.ExecutionResources))
	}

	return TransactionReceipt{
		FinalityStatus:  finalityStatus,
		ExecutionStatus: es,
		Type:            AdaptCoreTransaction(txn).Type,
		Hash:            txn.Hash(),
		ActualFee: &FeePayment{
			Amount: receipt.Fee,
			Unit:   feeUnit(txn),
		},
		BlockHash:          blockHash,
		BlockNumber:        receiptBlockNumber,
		MessagesSent:       messages,
		Events:             events,
		ContractAddress:    contractAddress,
		RevertReason:       receipt.RevertReason,
		ExecutionResources: executionResources,
		MessageHash:        messageHash,
	}
}

/****************************************************
		Trie Adapters
*****************************************************/

func adaptTrieProofNodes(proof *trie.ProofNodeSet) []*HashToNode {
	nodes := make([]*HashToNode, proof.Size())
	nodeList := proof.List()

	for i, hash := range proof.Keys() {
		var node Node

		switch n := nodeList[i].(type) {
		case *trie.Binary:
			node = &BinaryNode{
				Left:  n.LeftHash,
				Right: n.RightHash,
			}
		case *trie.Edge:
			path := n.Path.Felt()
			node = &EdgeNode{
				Path:   path.String(),
				Length: int(n.Path.Len()),
				Child:  n.Child,
			}
		}

		nodes[i] = &HashToNode{
			Hash: &hash,
			Node: node,
		}
	}

	return nodes
}

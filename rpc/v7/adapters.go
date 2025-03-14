package rpcv7

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/jinzhu/copier"
)

/**
 * Naming system:
 * - From another pkg to this pkg: adapt<FromPkg>TheType
 * - From this pkg to another: adaptTheTypeTo<ToPkg>
 */

/****************************************************
		VM Adapters
*****************************************************/

func AdaptVMTransactionTrace(trace *vm.TransactionTrace) TransactionTrace {
	var validateInvocation *rpcv6.FunctionInvocation
	if trace.ValidateInvocation != nil && trace.Type != vm.TxnL1Handler {
		validateInvocation = utils.HeapPtr(rpcv6.AdaptVMFunctionInvocation(trace.ValidateInvocation))
	}

	var feeTransferInvocation *rpcv6.FunctionInvocation
	if trace.FeeTransferInvocation != nil && trace.Type != vm.TxnL1Handler {
		feeTransferInvocation = utils.HeapPtr(rpcv6.AdaptVMFunctionInvocation(trace.FeeTransferInvocation))
	}

	var constructorInvocation *rpcv6.FunctionInvocation
	var executeInvocation *rpcv6.ExecuteInvocation
	var functionInvocation *rpcv6.FunctionInvocation

	switch trace.Type {
	case vm.TxnDeployAccount, vm.TxnDeploy:
		if trace.ConstructorInvocation != nil {
			constructorInvocation = utils.HeapPtr(rpcv6.AdaptVMFunctionInvocation(trace.ConstructorInvocation))
		}
	case vm.TxnInvoke:
		if trace.ExecuteInvocation != nil {
			executeInvocation = utils.HeapPtr(rpcv6.AdaptVMExecuteInvocation(trace.ExecuteInvocation))
		}
	case vm.TxnL1Handler:
		if trace.FunctionInvocation != nil {
			functionInvocation = utils.HeapPtr(rpcv6.AdaptVMFunctionInvocation(trace.FunctionInvocation))
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

func adaptVMExecutionResources(r *vm.ExecutionResources) ExecutionResources {
	// Adapt data availability
	var adaptedDataAvailability *DataAvailability
	if r.DataAvailability != nil {
		adaptedDataAvailability = &DataAvailability{
			L1Gas:     r.DataAvailability.L1Gas,
			L1DataGas: r.DataAvailability.L1DataGas,
		}
	}

	return ExecutionResources{
		ComputationResources: ComputationResources{
			Steps:        r.Steps,
			MemoryHoles:  r.MemoryHoles,
			Pedersen:     r.Pedersen,
			RangeCheck:   r.RangeCheck,
			Bitwise:      r.Bitwise,
			Ecdsa:        r.Ecdsa,
			EcOp:         r.EcOp,
			Keccak:       r.Keccak,
			Poseidon:     r.Poseidon,
			SegmentArena: r.SegmentArena,
		},
		DataAvailability: adaptedDataAvailability,
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

	// Adapt every feeder block trace to rpc v7 trace
	adaptedTraces := make([]TracedBlockTransaction, len(blockTrace.Traces))
	for index := range blockTrace.Traces {
		feederTrace := &blockTrace.Traces[index]

		trace := TransactionTrace{
			Type: block.Transactions[index].Type,
		}

		if fee := feederTrace.FeeTransferInvocation; fee != nil && trace.Type != TxnL1Handler {
			trace.FeeTransferInvocation = utils.HeapPtr(rpcv6.AdaptFeederFunctionInvocation(fee))
		}

		if val := feederTrace.ValidateInvocation; val != nil && trace.Type != TxnL1Handler {
			trace.ValidateInvocation = utils.HeapPtr(rpcv6.AdaptFeederFunctionInvocation(val))
		}

		var fnInvocation *rpcv6.FunctionInvocation
		if fct := feederTrace.FunctionInvocation; fct != nil {
			fnInvocation = utils.HeapPtr(rpcv6.AdaptFeederFunctionInvocation(fct))
		}

		switch trace.Type {
		case TxnDeploy, TxnDeployAccount:
			trace.ConstructorInvocation = fnInvocation
		case TxnInvoke:
			trace.ExecuteInvocation = new(rpcv6.ExecuteInvocation)
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

func adaptCoreBlockHeader(header *core.Header) rpcv6.BlockHeader {
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

	return rpcv6.BlockHeader{
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
	}
}

func nilToZero(f *felt.Felt) felt.Felt {
	if f == nil {
		return felt.Zero
	}
	return *f
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

func adaptCoreExecutionResources(resources *core.ExecutionResources) ExecutionResources {
	var dataAvailability *DataAvailability
	if resources.DataAvailability != nil {
		dataAvailability = &DataAvailability{
			L1Gas:     resources.DataAvailability.L1Gas,
			L1DataGas: resources.DataAvailability.L1DataGas,
		}
	}

	return ExecutionResources{
		ComputationResources: ComputationResources{
			Steps:        resources.Steps,
			MemoryHoles:  resources.MemoryHoles,
			Pedersen:     resources.BuiltinInstanceCounter.Pedersen,
			RangeCheck:   resources.BuiltinInstanceCounter.RangeCheck,
			Bitwise:      resources.BuiltinInstanceCounter.Bitwise,
			Ecdsa:        resources.BuiltinInstanceCounter.Ecsda,
			EcOp:         resources.BuiltinInstanceCounter.EcOp,
			Keccak:       resources.BuiltinInstanceCounter.Keccak,
			Poseidon:     resources.BuiltinInstanceCounter.Poseidon,
			SegmentArena: resources.BuiltinInstanceCounter.SegmentArena,
		},
		DataAvailability: dataAvailability,
	}
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
		txn = adaptCoreDeployAccountTransaction(v)
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

func adaptCoreDeployAccountTransaction(t *core.DeployAccountTransaction) Transaction {
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
		// ResourceL1DataGas is not supported in v7
		if resource == core.ResourceL1DataGas {
			continue
		}

		rpcResourceBounds[Resource(resource)] = ResourceBounds{
			MaxAmount:       new(felt.Felt).SetUint64(bounds.MaxAmount),
			MaxPricePerUnit: bounds.MaxPricePerUnit,
		}
	}

	return rpcResourceBounds
}

// todo(Kirill): try to replace core.Transaction with rpc.Transaction type
func AdaptCoreReceipt(receipt *core.TransactionReceipt, txn core.Transaction,
	finalityStatus TxnFinalityStatus, blockHash *felt.Felt, blockNumber uint64,
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

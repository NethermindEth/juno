package sn2core

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

func AdaptBlock(response *starknet.Block, sig *starknet.Signature) (*core.Block, error) {
	if response == nil {
		return nil, errors.New("nil client block")
	}

	txns := make([]core.Transaction, len(response.Transactions))
	for i := range response.Transactions {
		var err error
		txns[i], err = AdaptTransaction(response.Transactions[i])
		if err != nil {
			return nil, err
		}
	}

	receipts := make([]*core.TransactionReceipt, len(response.Receipts))
	eventCount := uint64(0)
	for i := range response.Receipts {
		receipts[i] = AdaptTransactionReceipt(response.Receipts[i])
		eventCount += uint64(len(response.Receipts[i].Events))
	}

	sigs := [][]*felt.Felt{}
	if sig != nil {
		sigs = append(sigs, sig.Signature)
	}

	return &core.Block{
		Header: &core.Header{
			Hash:             response.Hash,
			ParentHash:       response.ParentHash,
			Number:           response.Number,
			GlobalStateRoot:  response.StateRoot,
			Timestamp:        response.Timestamp,
			ProtocolVersion:  response.Version,
			SequencerAddress: response.SequencerAddress,
			TransactionCount: uint64(len(response.Transactions)),
			EventCount:       eventCount,
			EventsBloom:      core.EventsBloom(receipts),
			L1GasPriceETH:    response.L1GasPriceETH(),
			L1GasPriceSTRK:   response.L1GasPriceSTRK(),
			L1DAMode:         core.L1DAMode(response.L1DAMode),
			L1DataGasPrice:   (*core.GasPrice)(response.L1DataGasPrice),
			L2GasPrice:       (*core.GasPrice)(response.L2GasPrice),
			Signatures:       sigs,
		},
		Transactions: txns,
		Receipts:     receipts,
	}, nil
}

func AdaptTransactionReceipt(response *starknet.TransactionReceipt) *core.TransactionReceipt {
	if response == nil {
		return nil
	}

	return &core.TransactionReceipt{
		FeeUnit:            0, // todo(kirill) recheck
		Fee:                response.ActualFee,
		TransactionHash:    response.TransactionHash,
		Events:             utils.Map(utils.NonNilSlice(response.Events), AdaptEvent),
		ExecutionResources: AdaptExecutionResources(response.ExecutionResources),
		L1ToL2Message:      AdaptL1ToL2Message(response.L1ToL2Message),
		L2ToL1Message: utils.Map(
			utils.NonNilSlice(response.L2ToL1Message),
			AdaptL2ToL1Message,
		),
		Reverted:     response.ExecutionStatus == starknet.Reverted,
		RevertReason: response.RevertError,
	}
}

func adaptGasConsumed(response *starknet.GasConsumed) *core.GasConsumed {
	if response == nil {
		return nil
	}

	return &core.GasConsumed{
		L1Gas:     response.L1Gas,
		L1DataGas: response.L1DataGas,
		L2Gas:     response.L2Gas,
	}
}

func AdaptEvent(response *starknet.Event) *core.Event {
	if response == nil {
		return nil
	}

	return &core.Event{
		Data: response.Data,
		From: response.From,
		Keys: response.Keys,
	}
}

func AdaptExecutionResources(response *starknet.ExecutionResources) *core.ExecutionResources {
	if response == nil {
		return nil
	}

	return &core.ExecutionResources{
		BuiltinInstanceCounter: core.BuiltinInstanceCounter(response.BuiltinInstanceCounter),
		MemoryHoles:            response.MemoryHoles,
		Steps:                  response.Steps,
		DataAvailability:       (*core.DataAvailability)(response.DataAvailability),
		TotalGasConsumed:       adaptGasConsumed(response.TotalGasConsumed),
	}
}

func AdaptL1ToL2Message(response *starknet.L1ToL2Message) *core.L1ToL2Message {
	if response == nil {
		return nil
	}

	return &core.L1ToL2Message{
		From:     common.HexToAddress(response.From),
		Nonce:    response.Nonce,
		Payload:  response.Payload,
		Selector: response.Selector,
		To:       response.To,
	}
}

func AdaptL2ToL1Message(response *starknet.L2ToL1Message) *core.L2ToL1Message {
	if response == nil {
		return nil
	}

	return &core.L2ToL1Message{
		From:    response.From,
		Payload: response.Payload,
		To:      common.HexToAddress(response.To),
	}
}

func AdaptTransaction(transaction *starknet.Transaction) (core.Transaction, error) {
	txType := transaction.Type
	switch txType {
	case starknet.TxnDeclare:
		return AdaptDeclareTransaction(transaction), nil
	case starknet.TxnDeploy:
		return AdaptDeployTransaction(transaction), nil
	case starknet.TxnInvoke:
		return AdaptInvokeTransaction(transaction), nil
	case starknet.TxnDeployAccount:
		return AdaptDeployAccountTransaction(transaction), nil
	case starknet.TxnL1Handler:
		return AdaptL1HandlerTransaction(transaction), nil
	default:
		return nil, fmt.Errorf("unknown transaction type %q", txType)
	}
}

func AdaptDeclareTransaction(t *starknet.Transaction) *core.DeclareTransaction {
	return &core.DeclareTransaction{
		TransactionHash:       t.Hash,
		SenderAddress:         t.SenderAddress,
		MaxFee:                t.MaxFee,
		TransactionSignature:  *t.Signature,
		Nonce:                 t.Nonce,
		Version:               (*core.TransactionVersion)(t.Version),
		ClassHash:             t.ClassHash,
		CompiledClassHash:     t.CompiledClassHash,
		ResourceBounds:        adaptResourceBounds(t.ResourceBounds),
		Tip:                   safeFeltToUint64(t.Tip),
		PaymasterData:         utils.DerefSlice(t.PaymasterData),
		AccountDeploymentData: utils.DerefSlice(t.AccountDeploymentData),
		NonceDAMode:           adaptDataAvailabilityMode(t.NonceDAMode),
		FeeDAMode:             adaptDataAvailabilityMode(t.FeeDAMode),
	}
}

func adaptDataAvailabilityMode(mode *starknet.DataAvailabilityMode) core.DataAvailabilityMode {
	if mode == nil {
		return core.DAModeL1
	}
	return core.DataAvailabilityMode(*mode)
}

// todo(rdr): get rid of this gocritic
func adaptResourceBounds(
	rb *map[starknet.Resource]starknet.ResourceBounds, //nolint: gocritic // someone was lazy
) map[core.Resource]core.ResourceBounds {
	if rb == nil {
		return nil
	}
	coreBounds := make(map[core.Resource]core.ResourceBounds, len(*rb))
	for resource, bounds := range *rb {
		coreBounds[core.Resource(resource)] = core.ResourceBounds{
			MaxAmount:       bounds.MaxAmount.Uint64(),
			MaxPricePerUnit: bounds.MaxPricePerUnit,
		}
	}
	return coreBounds
}

// todo(rdr): return by value
func AdaptDeployTransaction(t *starknet.Transaction) *core.DeployTransaction {
	if t.ContractAddress == nil {
		t.ContractAddress = core.ContractAddress(
			&felt.Zero,
			t.ClassHash,
			t.ContractAddressSalt,
			*t.ConstructorCallData,
		)
	}
	return &core.DeployTransaction{
		TransactionHash:     t.Hash,
		ContractAddressSalt: t.ContractAddressSalt,
		ContractAddress:     t.ContractAddress,
		ClassHash:           t.ClassHash,
		ConstructorCallData: *t.ConstructorCallData,
		Version:             (*core.TransactionVersion)(t.Version),
	}
}

func AdaptInvokeTransaction(t *starknet.Transaction) *core.InvokeTransaction {
	return &core.InvokeTransaction{
		TransactionHash:       t.Hash,
		ContractAddress:       t.ContractAddress,
		EntryPointSelector:    t.EntryPointSelector,
		Nonce:                 t.Nonce,
		CallData:              *t.CallData,
		TransactionSignature:  *t.Signature,
		MaxFee:                t.MaxFee,
		Version:               (*core.TransactionVersion)(t.Version),
		SenderAddress:         t.SenderAddress,
		ResourceBounds:        adaptResourceBounds(t.ResourceBounds),
		Tip:                   safeFeltToUint64(t.Tip),
		PaymasterData:         utils.DerefSlice(t.PaymasterData),
		AccountDeploymentData: utils.DerefSlice(t.AccountDeploymentData),
		NonceDAMode:           adaptDataAvailabilityMode(t.NonceDAMode),
		FeeDAMode:             adaptDataAvailabilityMode(t.FeeDAMode),
	}
}

func AdaptL1HandlerTransaction(t *starknet.Transaction) *core.L1HandlerTransaction {
	return &core.L1HandlerTransaction{
		TransactionHash:    t.Hash,
		ContractAddress:    t.ContractAddress,
		EntryPointSelector: t.EntryPointSelector,
		Nonce:              t.Nonce,
		CallData:           *t.CallData,
		Version:            (*core.TransactionVersion)(t.Version),
	}
}

func AdaptDeployAccountTransaction(t *starknet.Transaction) *core.DeployAccountTransaction {
	return &core.DeployAccountTransaction{
		DeployTransaction:    *AdaptDeployTransaction(t),
		MaxFee:               t.MaxFee,
		TransactionSignature: *t.Signature,
		Nonce:                t.Nonce,
		ResourceBounds:       adaptResourceBounds(t.ResourceBounds),
		Tip:                  safeFeltToUint64(t.Tip),
		PaymasterData:        utils.DerefSlice(t.PaymasterData),
		NonceDAMode:          adaptDataAvailabilityMode(t.NonceDAMode),
		FeeDAMode:            adaptDataAvailabilityMode(t.FeeDAMode),
	}
}

func adaptSierraEntrypoint(ep *starknet.SierraEntryPoint) core.SierraEntryPoint {
	return core.SierraEntryPoint{
		Index:    ep.Index,
		Selector: ep.Selector,
	}
}

func adaptSierraEntrypoints(ep *starknet.SierraEntryPoints) core.SierraEntryPointsByType {
	entryPoints := core.SierraEntryPointsByType{
		Constructor: make([]core.SierraEntryPoint, len(ep.Constructor)),
		External:    make([]core.SierraEntryPoint, len(ep.External)),
		L1Handler:   make([]core.SierraEntryPoint, len(ep.L1Handler)),
	}

	for i := range ep.Constructor {
		entryPoints.Constructor[i] = adaptSierraEntrypoint(&ep.Constructor[i])
	}

	for i := range ep.External {
		entryPoints.External[i] = adaptSierraEntrypoint(&ep.External[i])
	}

	for i := range ep.L1Handler {
		entryPoints.L1Handler[i] = adaptSierraEntrypoint(&ep.L1Handler[i])
	}

	return entryPoints
}

func AdaptSierraClass(
	response *starknet.SierraClass,
	compiledClass *starknet.CasmClass,
) (*core.SierraClass, error) {
	var err error

	// TODO: what's the absolute minimum size of a Sierra Definition?
	// A Sierra program size should be at least 3 to contain the version or 1 if it's version is 0.1.0
	if len(response.Program) < 3 && (len(response.Program) == 0 ||
		!response.Program[0].Equal(&core.SierraVersion010)) {
		return nil, errors.New("sierra program size is too small")
	}

	coreCompiledClass, err := AdaptCompiledClass(compiledClass)
	if err != nil {
		return nil, err
	}

	return &core.SierraClass{
		SemanticVersion: response.Version,
		Program:         response.Program,
		ProgramHash:     crypto.PoseidonArray(response.Program...),

		Abi:     response.Abi,
		AbiHash: crypto.StarknetKeccak([]byte(response.Abi)),

		Compiled: &coreCompiledClass,

		EntryPoints: adaptSierraEntrypoints(&response.EntryPoints),
	}, nil
}

func AdaptCompiledClass(compiledClass *starknet.CasmClass) (core.CasmClass, error) {
	if compiledClass == nil {
		return core.CasmClass{}, nil
	}

	var casm core.CasmClass
	casm.Bytecode = compiledClass.Bytecode
	casm.PythonicHints = compiledClass.PythonicHints
	casm.CompilerVersion = compiledClass.CompilerVersion
	casm.Hints = compiledClass.Hints
	casm.BytecodeSegmentLengths = AdaptSegmentLengths(compiledClass.BytecodeSegmentLengths)

	var ok bool
	casm.Prime, ok = new(big.Int).SetString(compiledClass.Prime, 0)
	if !ok {
		return core.CasmClass{},
			fmt.Errorf("couldn't convert prime value to big.Int: %d", casm.Prime)
	}

	entryPoints := compiledClass.EntryPoints
	casm.External = utils.Map(entryPoints.External, adaptCompiledEntryPoint)
	casm.L1Handler = utils.Map(entryPoints.L1Handler, adaptCompiledEntryPoint)
	casm.Constructor = utils.Map(entryPoints.Constructor, adaptCompiledEntryPoint)

	return casm, nil
}

func AdaptSegmentLengths(l starknet.SegmentLengths) core.SegmentLengths {
	return core.SegmentLengths{
		Length:   l.Length,
		Children: utils.Map(l.Children, AdaptSegmentLengths),
	}
}

func adaptStarknetEntrypoint(ep *starknet.EntryPoint) core.DeprecatedEntryPoint {
	return core.DeprecatedEntryPoint{Selector: ep.Selector, Offset: ep.Offset}
}

// todo(rdr): We know the right type here which is a deprecated cairo class, why use polymorphism.
func AdaptDeprecatedCairoClass(
	response *starknet.DeprecatedCairoClass,
) (core.ClassDefinition, error) {
	class := new(core.DeprecatedCairoClass)
	class.Abi = response.Abi

	class.Externals = make([]core.DeprecatedEntryPoint, len(response.EntryPoints.External))
	for i := range response.EntryPoints.External {
		class.Externals[i] = adaptStarknetEntrypoint(&response.EntryPoints.External[i])
	}

	class.L1Handlers = make([]core.DeprecatedEntryPoint, len(response.EntryPoints.L1Handler))
	for i := range response.EntryPoints.L1Handler {
		class.L1Handlers[i] = adaptStarknetEntrypoint(&response.EntryPoints.L1Handler[i])
	}

	class.Constructors = make([]core.DeprecatedEntryPoint, len(response.EntryPoints.Constructor))
	for i := range response.EntryPoints.Constructor {
		class.Constructors[i] = adaptStarknetEntrypoint(&response.EntryPoints.Constructor[i])
	}

	var err error
	class.Program, err = utils.Gzip64Encode(response.Program)
	if err != nil {
		return nil, err
	}

	return class, nil
}

func AdaptStateUpdate(response *starknet.StateUpdate) (*core.StateUpdate, error) {
	stateDiff, err := AdaptStateDiff(&response.StateDiff)
	if err != nil {
		return nil, err
	}

	return &core.StateUpdate{
		BlockHash: response.BlockHash,
		NewRoot:   response.NewRoot,
		OldRoot:   response.OldRoot,
		StateDiff: &stateDiff,
	}, nil
}

func AdaptStateDiff(response *starknet.StateDiff) (core.StateDiff, error) {
	var stateDiff core.StateDiff
	stateDiff.DeclaredV0Classes = response.OldDeclaredContracts

	stateDiff.DeclaredV1Classes = make(map[felt.Felt]*felt.Felt, len(response.DeclaredClasses))
	for _, declaredV1Class := range response.DeclaredClasses {
		stateDiff.DeclaredV1Classes[*declaredV1Class.ClassHash] = declaredV1Class.CompiledClassHash
	}

	stateDiff.MigratedClasses = make(
		map[felt.SierraClassHash]felt.CasmClassHash, len(response.MigratedClasses),
	)
	for _, migratedClass := range response.MigratedClasses {
		stateDiff.MigratedClasses[migratedClass.ClassHash] = migratedClass.CompiledClassHash
	}

	stateDiff.ReplacedClasses = make(map[felt.Felt]*felt.Felt, len(response.ReplacedClasses))
	for _, replacedClass := range response.ReplacedClasses {
		stateDiff.ReplacedClasses[*replacedClass.Address] = replacedClass.ClassHash
	}

	stateDiff.DeployedContracts = make(map[felt.Felt]*felt.Felt, len(response.DeployedContracts))
	for _, deployedContract := range response.DeployedContracts {
		stateDiff.DeployedContracts[*deployedContract.Address] = deployedContract.ClassHash
	}

	stateDiff.Nonces = make(map[felt.Felt]*felt.Felt, len(response.Nonces))
	for addrStr, nonce := range response.Nonces {
		addr, err := new(felt.Felt).SetString(addrStr)
		if err != nil {
			return core.StateDiff{}, err
		}
		stateDiff.Nonces[*addr] = nonce
	}

	stateDiff.StorageDiffs = make(
		map[felt.Felt]map[felt.Felt]*felt.Felt,
		len(response.StorageDiffs),
	)
	for addrStr, diffs := range response.StorageDiffs {
		addr, err := new(felt.Felt).SetString(addrStr)
		if err != nil {
			return core.StateDiff{}, err
		}

		stateDiff.StorageDiffs[*addr] = make(map[felt.Felt]*felt.Felt)
		for _, diff := range diffs {
			stateDiff.StorageDiffs[*addr][*diff.Key] = diff.Value
		}
	}

	return stateDiff, nil
}

// Comparing to preconfirmed, candidate txns don't include state diffs and receipts.
// `||` is used to cover any possible descrepancies
// https://community.starknet.io/t/sn-0-14-0-pre-release-notes
func IsCandidateTx(response *starknet.PreConfirmedBlock, id int) bool {
	return response.TransactionStateDiffs[id] == nil || response.Receipts[id] == nil
}

func AdaptPreConfirmedBlock(
	response *starknet.PreConfirmedBlock,
	number uint64,
) (core.PreConfirmed, error) {
	if response == nil {
		return core.PreConfirmed{}, errors.New("nil preconfirmed block")
	}

	if response.Status != "PRE_CONFIRMED" {
		return core.PreConfirmed{}, errors.New("invalid status for pre_confirmed block")
	}

	isInvalidPayloadSizes := len(response.Transactions) != len(response.TransactionStateDiffs) ||
		len(response.Transactions) != len(response.Receipts)
	if isInvalidPayloadSizes {
		return core.PreConfirmed{}, errors.New("invalid sizes of transactions, state diffs and receipts")
	}

	preConfirmedTxCount := 0
	for i := range len(response.Transactions) {
		if !IsCandidateTx(response, i) {
			preConfirmedTxCount++
		}
	}
	candidateCount := len(response.Transactions) - preConfirmedTxCount

	txns := make([]core.Transaction, preConfirmedTxCount)
	txStateDiffs := make([]*core.StateDiff, preConfirmedTxCount)
	receipts := make([]*core.TransactionReceipt, preConfirmedTxCount)
	eventCount := uint64(0)
	candidateTxs := make([]core.Transaction, candidateCount)

	var err error
	preIdx := 0
	candIdx := 0
	for i := range len(response.Transactions) {
		if !IsCandidateTx(response, i) {
			txns[preIdx], err = AdaptTransaction(&response.Transactions[i])
			if err != nil {
				return core.PreConfirmed{}, err
			}
			var stateDiff core.StateDiff
			stateDiff, err = AdaptStateDiff(response.TransactionStateDiffs[i])
			if err != nil {
				return core.PreConfirmed{}, err
			}
			txStateDiffs[preIdx] = &stateDiff
			receipts[preIdx] = AdaptTransactionReceipt(response.Receipts[i])
			eventCount += uint64(len(response.Receipts[i].Events))
			preIdx++
		} else {
			candidateTxs[candIdx], err = AdaptTransaction(&response.Transactions[i])
			if err != nil {
				return core.PreConfirmed{}, err
			}
			candIdx++
		}
	}

	// Squash per-tx state updates
	stateDiff := core.EmptyStateDiff()
	for _, txStateDiff := range txStateDiffs {
		stateDiff.Merge(txStateDiff)
	}

	stateUpdate := core.StateUpdate{
		BlockHash: nil,
		NewRoot:   nil,
		// Must be set to previous global state root, when have access to latest header
		OldRoot:   nil,
		StateDiff: &stateDiff,
	}

	adaptedBlock := &core.Block{
		// https://github.com/starkware-libs/starknet-specs/blob/9377851884da5c81f757b6ae0ed47e84f9e7c058/api/starknet_api_openrpc.json#L1636
		Header: &core.Header{
			Number:           number,
			SequencerAddress: response.SequencerAddress,
			// Not required in spec but useful
			TransactionCount: uint64(len(txns)),
			// Not required in spec but useful
			EventCount:      eventCount,
			Timestamp:       response.Timestamp,
			ProtocolVersion: response.Version,
			// Not required in spec but useful
			EventsBloom:    core.EventsBloom(receipts),
			L1GasPriceETH:  response.L1GasPrice.PriceInWei,
			L1GasPriceSTRK: response.L1GasPrice.PriceInFri,
			L1DAMode:       core.L1DAMode(response.L1DAMode),
			L1DataGasPrice: (*core.GasPrice)(response.L1DataGasPrice),
			L2GasPrice:     (*core.GasPrice)(response.L2GasPrice),
			// Following fields are nil for pre_confirmed block
			Hash:            nil,
			ParentHash:      nil,
			GlobalStateRoot: nil,
			Signatures:      nil,
		},
		Transactions: txns,
		Receipts:     receipts,
	}
	return core.NewPreConfirmed(adaptedBlock, &stateUpdate, txStateDiffs, candidateTxs), nil
}

func safeFeltToUint64(f *felt.Felt) uint64 {
	if f != nil {
		return f.Uint64()
	}
	return 0
}

func adaptCompiledEntryPoint(entryPoint starknet.CompiledEntryPoint) core.CasmEntryPoint {
	return core.CasmEntryPoint{
		Offset:   entryPoint.Offset,
		Selector: entryPoint.Selector,
		Builtins: entryPoint.Builtins,
	}
}

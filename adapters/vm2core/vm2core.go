package vm2core

import (
	"cmp"
	"slices"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

func AdaptOrderedEvent(event vm.OrderedEvent) *core.Event {
	return &core.Event{
		From: event.From,
		Keys: event.Keys,
		Data: event.Data,
	}
}

// todo(rdr): this is function definition is twice wrong:
//   - the parameters should be received by reference
//   - the output param should be return by value
func AdaptOrderedMessageToL1(message *vm.OrderedL2toL1Message) *core.L2ToL1Message {
	var to types.L1Address
	if message.To != nil {
		toBytes := message.To.Marshal()
		to = types.FromBytes[types.L1Address](toBytes)
	}
	return &core.L2ToL1Message{
		// todo: remove this cast
		From:    (*felt.Address)(message.From),
		Payload: message.Payload,
		To:      to,
	}
}

func AdaptOrderedMessagesToL1(messages []vm.OrderedL2toL1Message) []*core.L2ToL1Message {
	slices.SortFunc(messages, func(a, b vm.OrderedL2toL1Message) int {
		return cmp.Compare(a.Order, b.Order)
	})

	out := make([]*core.L2ToL1Message, len(messages))
	for i := range messages {
		m := AdaptOrderedMessageToL1(&messages[i])
		out[i] = m
	}
	return out
}

func AdaptOrderedEvents(events []vm.OrderedEvent) []*core.Event {
	slices.SortFunc(events, func(a, b vm.OrderedEvent) int {
		return cmp.Compare(a.Order, b.Order)
	})
	return utils.Map(events, AdaptOrderedEvent)
}

func AdaptStateDiff(fromStateDiff *vm.StateDiff) core.StateDiff {
	var toStateDiff core.StateDiff
	if fromStateDiff == nil {
		return toStateDiff
	}

	// Preallocate all maps with known sizes from fromStateDiff
	toStateDiff = core.StateDiff{
		StorageDiffs: make(
			map[felt.Felt]map[felt.Felt]*felt.Felt, len(fromStateDiff.StorageDiffs),
		),
		Nonces:            make(map[felt.Felt]*felt.Felt, len(fromStateDiff.Nonces)),
		DeployedContracts: make(map[felt.Felt]*felt.Felt, len(fromStateDiff.DeployedContracts)),
		DeclaredV0Classes: make([]*felt.Felt, len(fromStateDiff.DeprecatedDeclaredClasses)),
		DeclaredV1Classes: make(map[felt.Felt]*felt.Felt, len(fromStateDiff.DeclaredClasses)),
		MigratedClasses: make(
			map[felt.SierraClassHash]felt.CasmClassHash,
			len(fromStateDiff.MigratedCompiledClasses),
		),
		ReplacedClasses: make(map[felt.Felt]*felt.Felt, len(fromStateDiff.ReplacedClasses)),
	}

	for _, sd := range fromStateDiff.StorageDiffs {
		entries := make(map[felt.Felt]*felt.Felt, len(sd.StorageEntries))
		for _, entry := range sd.StorageEntries {
			val := entry.Value
			entries[entry.Key] = &val
		}
		toStateDiff.StorageDiffs[sd.Address] = entries
	}

	for _, nonce := range fromStateDiff.Nonces {
		newNonce := nonce.Nonce
		toStateDiff.Nonces[nonce.ContractAddress] = &newNonce
	}

	for _, dc := range fromStateDiff.DeployedContracts {
		ch := dc.ClassHash
		toStateDiff.DeployedContracts[dc.Address] = &ch
	}

	for _, dc := range fromStateDiff.DeclaredClasses {
		cch := dc.CompiledClassHash
		toStateDiff.DeclaredV1Classes[dc.ClassHash] = &cch
	}

	for _, rc := range fromStateDiff.ReplacedClasses {
		ch := rc.ClassHash
		toStateDiff.ReplacedClasses[rc.ContractAddress] = &ch
	}

	for _, mc := range fromStateDiff.MigratedCompiledClasses {
		toStateDiff.MigratedClasses[mc.ClassHash] = mc.CompiledClassHash
	}

	return toStateDiff
}

func Receipt(fee *felt.Felt, txn core.Transaction,
	trace *vm.TransactionTrace, txnReceipt *vm.TransactionReceipt,
) core.TransactionReceipt {
	// Determine fee unit based on transaction version
	feeUnit := core.WEI
	if txn.TxVersion().Is(3) {
		feeUnit = core.STRK
	}

	adaptedER := AdaptExecutionResources(trace.TotalExecutionResources(), &txnReceipt.Gas)
	isReverted := trace.ExecuteInvocation != nil && trace.ExecuteInvocation.FunctionInvocation == nil
	return core.TransactionReceipt{
		Fee:                fee,
		FeeUnit:            feeUnit,
		Events:             AdaptOrderedEvents(trace.AllEvents()),
		ExecutionResources: &adaptedER,
		L1ToL2Message:      nil, // Todo: sequencer currently can't post messages to L1
		L2ToL1Message:      AdaptOrderedMessagesToL1(trace.AllMessages()),
		TransactionHash:    txn.Hash(),
		Reverted:           isReverted,
		RevertReason:       trace.RevertReason(),
	}
}

func AdaptExecutionResources(resources *vm.ExecutionResources, totalGas *vm.GasConsumed) core.ExecutionResources {
	adaptedDA := adaptDA(resources.DataAvailability)
	return core.ExecutionResources{
		BuiltinInstanceCounter: core.BuiltinInstanceCounter{
			Pedersen:     resources.Pedersen,
			RangeCheck:   resources.RangeCheck,
			Bitwise:      resources.Bitwise,
			Ecsda:        resources.Ecdsa,
			EcOp:         resources.EcOp,
			Keccak:       resources.Keccak,
			Poseidon:     resources.Poseidon,
			SegmentArena: resources.SegmentArena,
			Output:       resources.Output,
			AddMod:       resources.AddMod,
			MulMod:       resources.MulMod,
			RangeCheck96: resources.RangeCheck96,
		},
		MemoryHoles:      resources.MemoryHoles,
		Steps:            resources.Steps,
		DataAvailability: &adaptedDA,
		TotalGasConsumed: &core.GasConsumed{L1Gas: totalGas.L1Gas, L1DataGas: totalGas.L1DataGas, L2Gas: 0}, // Todo
	}
}

func adaptDA(da *vm.DataAvailability) core.DataAvailability {
	if da == nil {
		return core.DataAvailability{
			L1Gas:     0,
			L1DataGas: 0,
		}
	}

	return core.DataAvailability{
		L1Gas:     da.L1Gas,
		L1DataGas: da.L1DataGas,
	}
}

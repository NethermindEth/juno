package p2p2core

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

// todo change type of txHash to spec
func AdaptReceipt(r *spec.Receipt, txHash *felt.Felt) *core.TransactionReceipt {
	var common *spec.Receipt_Common

	switch r.Type.(type) {
	case *spec.Receipt_Invoke_:
		common = r.GetInvoke().GetCommon()
	case *spec.Receipt_Declare_:
		common = r.GetDeclare().GetCommon()
	case *spec.Receipt_DeployAccount_:
		common = r.GetDeployAccount().GetCommon()
	case *spec.Receipt_L1Handler_:
		common = r.GetL1Handler().GetCommon()
	case *spec.Receipt_DeprecatedDeploy:
		common = r.GetDeprecatedDeploy().GetCommon()
	}

	return &core.TransactionReceipt{
		Fee:                AdaptFelt(common.ActualFee),
		FeeUnit:            0,   // todo(kirill) recheck
		Events:             nil, // todo SPEC , current specification does not maintain the mapping of events to transactions receipts
		ExecutionResources: adaptExecutionResources(common.ExecutionResources),
		L1ToL2Message:      nil,
		L2ToL1Message:      utils.Map(common.MessagesSent, adaptMessageToL1),
		TransactionHash:    txHash,
		Reverted:           common.RevertReason != nil, // in case it's empty string we should treat it as reverted
		RevertReason:       common.GetRevertReason(),
	}
}

func adaptExecutionResources(er *spec.Receipt_ExecutionResources) *core.ExecutionResources {
	if er == nil {
		return nil
	}
	return &core.ExecutionResources{
		BuiltinInstanceCounter: core.BuiltinInstanceCounter{
			Pedersen:     uint64(er.GetBuiltins().GetPedersen()),
			RangeCheck:   uint64(er.GetBuiltins().GetRangeCheck()),
			Bitwise:      uint64(er.GetBuiltins().GetBitwise()),
			Output:       uint64(er.GetBuiltins().GetOutput()),
			Ecsda:        uint64(er.GetBuiltins().GetEcdsa()),
			EcOp:         uint64(er.GetBuiltins().GetEcOp()),
			Keccak:       uint64(er.GetBuiltins().GetKeccak()),
			Poseidon:     uint64(er.GetBuiltins().GetPoseidon()),
			SegmentArena: 0, // todo(kirill) recheck
			AddMod:       uint64(er.GetBuiltins().GetAddMod()),
			MulMod:       uint64(er.GetBuiltins().GetMulMod()),
			RangeCheck96: uint64(er.GetBuiltins().GetRangeCheck96()),
		},
		DataAvailability: &core.DataAvailability{
			L1Gas:     feltToUint64(er.L1Gas),
			L1DataGas: feltToUint64(er.L1DataGas),
		},
		MemoryHoles: uint64(er.MemoryHoles),
		Steps:       uint64(er.Steps), // todo SPEC 32 -> 64 bytes
		TotalGasConsumed: &core.GasConsumed{
			L1Gas: feltToUint64(er.TotalL1Gas),
			// total_l1_data_gas = l1_data_gas, because there's only one place that can generate l1_data_gas costs
			L1DataGas: feltToUint64(er.L1DataGas),
		},
	}
}

func adaptMessageToL1(m *spec.MessageToL1) *core.L2ToL1Message {
	return &core.L2ToL1Message{
		From:    AdaptFelt(m.FromAddress),
		To:      AdaptEthAddress(m.ToAddress),
		Payload: utils.Map(m.Payload, AdaptFelt),
	}
}

func feltToUint64(f *spec.Felt252) uint64 {
	var result uint64
	if adapted := AdaptFelt(f); adapted != nil {
		result = adapted.Uint64()
	}
	return result
}

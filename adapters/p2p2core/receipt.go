package p2p2core

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/receipt"
)

// todo change type of txHash to spec
func AdaptReceipt(r *receipt.Receipt, txHash *felt.Felt) *core.TransactionReceipt {
	var common *receipt.Receipt_Common

	switch r.Type.(type) {
	case *receipt.Receipt_Invoke_:
		common = r.GetInvoke().GetCommon()
	case *receipt.Receipt_Declare_:
		common = r.GetDeclare().GetCommon()
	case *receipt.Receipt_DeployAccount_:
		common = r.GetDeployAccount().GetCommon()
	case *receipt.Receipt_L1Handler_:
		common = r.GetL1Handler().GetCommon()
	case *receipt.Receipt_DeprecatedDeploy:
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

func adaptExecutionResources(er *receipt.Receipt_ExecutionResources) *core.ExecutionResources {
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
			L2Gas: feltToUint64(er.L2Gas),
			// total_l1_data_gas = l1_data_gas, because there's only one place that can generate l1_data_gas costs
			L1DataGas: feltToUint64(er.L1DataGas),
		},
	}
}

func adaptMessageToL1(m *receipt.MessageToL1) *core.L2ToL1Message {
	from := (*felt.Address)(AdaptFelt(m.FromAddress))
	var to *types.L1Address
	if m.ToAddress != nil {
		to = types.NewFromBytes[types.L1Address](m.ToAddress.GetElements())
	}
	return &core.L2ToL1Message{
		From:    from,
		To:      to,
		Payload: utils.Map(m.Payload, AdaptFelt),
	}
}

func feltToUint64(f *common.Felt252) uint64 {
	var result uint64
	if adapted := AdaptFelt(f); adapted != nil {
		result = adapted.Uint64()
	}
	return result
}

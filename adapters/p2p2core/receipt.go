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
		Reverted:           common.RevertReason != nil, // todo is it correct?
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
			// todo(kirill) set fields after spec update
			AddMod:       0,
			MulMod:       0,
			RangeCheck96: 0,
		},
		DataAvailability: nil, // todo(kirill) recheck
		MemoryHoles:      uint64(er.MemoryHoles),
		Steps:            uint64(er.Steps), // todo SPEC 32 -> 64 bytes
		TotalGasConsumed: nil,              // todo(kirill) fill after spec update
	}
}

func adaptMessageToL1(m *spec.MessageToL1) *core.L2ToL1Message {
	return &core.L2ToL1Message{
		From:    AdaptFelt(m.FromAddress),
		To:      AdaptEthAddress(m.ToAddress),
		Payload: utils.Map(m.Payload, AdaptFelt),
	}
}

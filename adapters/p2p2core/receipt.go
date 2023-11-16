package p2p2core

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func AdaptReceipt(r *spec.Receipt) *core.TransactionReceipt {
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
		Events:             nil, // todo SPEC , current specification does not maintain the mapping of events to transactions receipts
		ExecutionResources: adaptExecutionResources(common.ExecutionResources),
		L1ToL2Message:      nil,
		L2ToL1Message:      utils.Map(common.MessagesSent, adaptMessageToL1),
		TransactionHash:    AdaptHash(common.TransactionHash),
		Reverted:           common.RevertReason != "", // todo is it correct?
		RevertReason:       common.RevertReason,
	}
}

func adaptExecutionResources(er *spec.Receipt_ExecutionResources) *core.ExecutionResources {
	if er == nil {
		return nil
	}
	return &core.ExecutionResources{
		BuiltinInstanceCounter: core.BuiltinInstanceCounter{
			Pedersen:   uint64(er.GetBuiltins().GetPedersen()),
			RangeCheck: uint64(er.GetBuiltins().GetRangeCheck()),
			Bitwise:    uint64(er.GetBuiltins().GetBitwise()),
			Output:     0, // todo SPEC
			Ecsda:      uint64(er.GetBuiltins().GetEcdsa()),
			EcOp:       uint64(er.GetBuiltins().GetEcOp()),
			Keccak:     uint64(er.GetBuiltins().GetKeccak()),
			Poseidon:   uint64(er.GetBuiltins().GetPoseidon()),
		},
		MemoryHoles: uint64(er.MemoryHoles),
		Steps:       uint64(er.Steps), // todo SPEC 32 -> 64 bytes
	}
}

func adaptMessageToL1(m *spec.MessageToL1) *core.L2ToL1Message {
	return &core.L2ToL1Message{
		From:    AdaptFelt(m.FromAddress),
		Payload: utils.Map(m.Payload, AdaptFelt),
		To:      AdaptEthAddress(m.ToAddress),
	}
}

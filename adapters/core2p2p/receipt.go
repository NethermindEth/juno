package core2p2p

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

// Core Transaction receipt does not contain all the information required to create p2p spec Receipt, therefore,
// we have to pass the transaction as well.
func AdaptReceipt(r *core.TransactionReceipt, txn core.Transaction) *spec.Receipt {
	if r == nil || txn == nil {
		return nil
	}
	switch t := txn.(type) {
	case *core.InvokeTransaction:
		return &spec.Receipt{
			Receipt: &spec.Receipt_Invoke_{
				Invoke: &spec.Receipt_Invoke{
					Common: receiptCommon(r),
				},
			},
		}
	case *core.L1HandlerTransaction:
		return &spec.Receipt{
			Receipt: &spec.Receipt_L1Handler_{
				L1Handler: &spec.Receipt_L1Handler{
					Common:  receiptCommon(r),
					MsgHash: &spec.Hash{Elements: t.MessageHash()},
				},
			},
		}
	case *core.DeclareTransaction:
		return &spec.Receipt{
			Receipt: &spec.Receipt_Declare_{
				Declare: &spec.Receipt_Declare{
					Common: receiptCommon(r),
				},
			},
		}
	case *core.DeployTransaction:
		return &spec.Receipt{
			Receipt: &spec.Receipt_DeprecatedDeploy{
				DeprecatedDeploy: &spec.Receipt_Deploy{
					Common:          receiptCommon(r),
					ContractAddress: AdaptFelt(t.ContractAddress),
				},
			},
		}
	case *core.DeployAccountTransaction:
		return &spec.Receipt{
			Receipt: &spec.Receipt_DeployAccount_{
				DeployAccount: &spec.Receipt_DeployAccount{
					Common:          receiptCommon(r),
					ContractAddress: AdaptFelt(t.ContractAddress),
				},
			},
		}
	default:
		return nil
	}
}

func receiptCommon(r *core.TransactionReceipt) *spec.Receipt_Common {
	return &spec.Receipt_Common{
		TransactionHash:    AdaptHash(r.TransactionHash),
		ActualFee:          AdaptFelt(r.Fee),
		MessagesSent:       utils.Map(r.L2ToL1Message, AdaptMessageToL1),
		ExecutionResources: AdaptExecutionResources(r.ExecutionResources),
		RevertReason:       r.RevertReason,
	}
}

func AdaptMessageToL1(mL1 *core.L2ToL1Message) *spec.MessageToL1 {
	return &spec.MessageToL1{
		FromAddress: AdaptFelt(mL1.From),
		Payload:     utils.Map(mL1.Payload, AdaptFelt),
		ToAddress:   &spec.EthereumAddress{Elements: mL1.To.Bytes()},
	}
}

func AdaptExecutionResources(er *core.ExecutionResources) *spec.Receipt_ExecutionResources {
	return &spec.Receipt_ExecutionResources{
		Builtins: &spec.Receipt_ExecutionResources_BuiltinCounter{
			Bitwise:    uint32(er.BuiltinInstanceCounter.Bitwise),
			Ecdsa:      uint32(er.BuiltinInstanceCounter.Ecsda),
			EcOp:       uint32(er.BuiltinInstanceCounter.EcOp),
			Pedersen:   uint32(er.BuiltinInstanceCounter.Pedersen),
			RangeCheck: uint32(er.BuiltinInstanceCounter.RangeCheck),
			Poseidon:   uint32(er.BuiltinInstanceCounter.Poseidon),
			Keccak:     uint32(er.BuiltinInstanceCounter.Keccak),
		},
		Steps:       uint32(er.Steps),
		MemoryHoles: uint32(er.MemoryHoles),
	}
}

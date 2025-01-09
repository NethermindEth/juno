package core2p2p

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/gen"
	"github.com/NethermindEth/juno/utils"
)

// Core Transaction receipt does not contain all the information required to create p2p spec Receipt, therefore,
// we have to pass the transaction as well.
func AdaptReceipt(r *core.TransactionReceipt, txn core.Transaction) *gen.Receipt {
	if r == nil || txn == nil {
		return nil
	}
	switch t := txn.(type) {
	case *core.InvokeTransaction:
		return &gen.Receipt{
			Type: &gen.Receipt_Invoke_{
				Invoke: &gen.Receipt_Invoke{
					Common: receiptCommon(r),
				},
			},
		}
	case *core.L1HandlerTransaction:
		return &gen.Receipt{
			Type: &gen.Receipt_L1Handler_{
				L1Handler: &gen.Receipt_L1Handler{
					Common:  receiptCommon(r),
					MsgHash: &gen.Hash256{Elements: t.MessageHash()},
				},
			},
		}
	case *core.DeclareTransaction:
		return &gen.Receipt{
			Type: &gen.Receipt_Declare_{
				Declare: &gen.Receipt_Declare{
					Common: receiptCommon(r),
				},
			},
		}
	case *core.DeployTransaction:
		return &gen.Receipt{
			Type: &gen.Receipt_DeprecatedDeploy{
				DeprecatedDeploy: &gen.Receipt_Deploy{
					Common:          receiptCommon(r),
					ContractAddress: AdaptFelt(t.ContractAddress),
				},
			},
		}
	case *core.DeployAccountTransaction:
		return &gen.Receipt{
			Type: &gen.Receipt_DeployAccount_{
				DeployAccount: &gen.Receipt_DeployAccount{
					Common:          receiptCommon(r),
					ContractAddress: AdaptFelt(t.ContractAddress),
				},
			},
		}
	default:
		return nil
	}
}

func receiptCommon(r *core.TransactionReceipt) *gen.Receipt_Common {
	var revertReason *string
	if r.RevertReason != "" {
		revertReason = &r.RevertReason
	} else if r.Reverted {
		// in some cases receipt marked as reverted may contain empty string in revert_reason
		revertReason = utils.Ptr("")
	}

	return &gen.Receipt_Common{
		ActualFee:          AdaptFelt(r.Fee),
		PriceUnit:          adaptPriceUnit(r.FeeUnit),
		MessagesSent:       utils.Map(r.L2ToL1Message, AdaptMessageToL1),
		ExecutionResources: AdaptExecutionResources(r.ExecutionResources),
		RevertReason:       revertReason,
	}
}

func adaptPriceUnit(unit core.FeeUnit) gen.PriceUnit {
	switch unit {
	case core.WEI:
		return gen.PriceUnit_Wei
	case core.STRK:
		return gen.PriceUnit_Fri // todo(kirill) recheck
	default:
		panic("unreachable adaptPriceUnit")
	}
}

func AdaptMessageToL1(mL1 *core.L2ToL1Message) *gen.MessageToL1 {
	return &gen.MessageToL1{
		FromAddress: AdaptFelt(mL1.From),
		Payload:     utils.Map(mL1.Payload, AdaptFelt),
		ToAddress:   &gen.EthereumAddress{Elements: mL1.To.Bytes()},
	}
}

func AdaptExecutionResources(er *core.ExecutionResources) *gen.Receipt_ExecutionResources {
	if er == nil {
		return nil
	}

	var l1Gas, l1DataGas, totalL1Gas *felt.Felt
	if da := er.DataAvailability; da != nil { // todo(kirill) check that it might be null
		l1Gas = new(felt.Felt).SetUint64(da.L1Gas)
		l1DataGas = new(felt.Felt).SetUint64(da.L1DataGas)
	}
	if tgs := er.TotalGasConsumed; tgs != nil {
		totalL1Gas = new(felt.Felt).SetUint64(tgs.L1Gas)
	}

	return &gen.Receipt_ExecutionResources{
		Builtins: &gen.Receipt_ExecutionResources_BuiltinCounter{
			Bitwise:      uint32(er.BuiltinInstanceCounter.Bitwise),
			Ecdsa:        uint32(er.BuiltinInstanceCounter.Ecsda),
			EcOp:         uint32(er.BuiltinInstanceCounter.EcOp),
			Pedersen:     uint32(er.BuiltinInstanceCounter.Pedersen),
			RangeCheck:   uint32(er.BuiltinInstanceCounter.RangeCheck),
			Poseidon:     uint32(er.BuiltinInstanceCounter.Poseidon),
			Keccak:       uint32(er.BuiltinInstanceCounter.Keccak),
			Output:       uint32(er.BuiltinInstanceCounter.Output),
			AddMod:       uint32(er.BuiltinInstanceCounter.AddMod),
			MulMod:       uint32(er.BuiltinInstanceCounter.MulMod),
			RangeCheck96: uint32(er.BuiltinInstanceCounter.RangeCheck96),
		},
		Steps:       uint32(er.Steps),
		MemoryHoles: uint32(er.MemoryHoles),
		L1Gas:       AdaptFelt(l1Gas),
		L1DataGas:   AdaptFelt(l1DataGas),
		TotalL1Gas:  AdaptFelt(totalL1Gas),
	}
}

package core2p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
)

//nolint:funlen,gocyclo
func AdaptTransaction(transaction core.Transaction) *spec.Transaction {
	if transaction == nil {
		return nil
	}

	var specTx spec.Transaction

	switch tx := transaction.(type) {
	case *core.DeployTransaction:
		specTx.Txn = adaptDeployTransaction(tx)
	case *core.DeployAccountTransaction:
		switch {
		case tx.Version.Is(1):
			specTx.Txn = &spec.Transaction_DeployAccountV1_{
				DeployAccountV1: &spec.Transaction_DeployAccountV1{
					MaxFee:      AdaptFelt(tx.MaxFee),
					Signature:   AdaptAccountSignature(tx.Signature()),
					ClassHash:   AdaptHash(tx.ClassHash),
					Nonce:       AdaptFelt(tx.Nonce),
					AddressSalt: AdaptFelt(tx.ContractAddressSalt),
					Calldata:    AdaptFeltSlice(tx.ConstructorCallData),
				},
			}
		case tx.Version.Is(3):
			specTx.Txn = &spec.Transaction_DeployAccountV3_{
				DeployAccountV3: &spec.Transaction_DeployAccountV3{
					MaxFee:      AdaptFelt(tx.MaxFee),
					Signature:   AdaptAccountSignature(tx.Signature()),
					ClassHash:   AdaptHash(tx.ClassHash),
					Nonce:       AdaptFelt(tx.Nonce),
					AddressSalt: AdaptFelt(tx.ContractAddressSalt),
					Calldata:    AdaptFeltSlice(tx.ConstructorCallData),
					L1Gas:       adaptResourceLimits(tx.ResourceBounds[core.ResourceL1Gas]),
					L2Gas:       adaptResourceLimits(tx.ResourceBounds[core.ResourceL2Gas]),
					Tip:         AdaptFelt(new(felt.Felt).SetUint64(tx.Tip)),
					// todo fill rest of V3 fields:
					Paymaster:   nil,
					NonceDomain: "",
					FeeDomain:   "",
				},
			}
		default:
			panic(fmt.Errorf("unsupported InvokeTransaction version %s", tx.Version))
		}
	case *core.DeclareTransaction:
		switch {
		case tx.Version.Is(0):
			specTx.Txn = &spec.Transaction_DeclareV0_{
				DeclareV0: &spec.Transaction_DeclareV0{
					Sender:    AdaptAddress(tx.SenderAddress),
					MaxFee:    AdaptFelt(tx.MaxFee),
					Signature: AdaptAccountSignature(tx.Signature()),
					ClassHash: AdaptHash(tx.ClassHash),
				},
			}
		case tx.Version.Is(1):
			specTx.Txn = &spec.Transaction_DeclareV1_{
				DeclareV1: &spec.Transaction_DeclareV1{
					Sender:    AdaptAddress(tx.SenderAddress),
					MaxFee:    AdaptFelt(tx.MaxFee),
					Signature: AdaptAccountSignature(tx.Signature()),
					ClassHash: AdaptHash(tx.ClassHash),
					Nonce:     AdaptFelt(tx.Nonce),
				},
			}
		case tx.Version.Is(2):
			specTx.Txn = &spec.Transaction_DeclareV2_{
				DeclareV2: &spec.Transaction_DeclareV2{
					Sender:            AdaptAddress(tx.SenderAddress),
					MaxFee:            AdaptFelt(tx.MaxFee),
					Signature:         AdaptAccountSignature(tx.Signature()),
					ClassHash:         AdaptHash(tx.ClassHash),
					Nonce:             AdaptFelt(tx.Nonce),
					CompiledClassHash: AdaptFelt(tx.CompiledClassHash),
				},
			}
		case tx.Version.Is(3):
			specTx.Txn = &spec.Transaction_DeclareV3_{
				DeclareV3: &spec.Transaction_DeclareV3{
					Sender:            AdaptAddress(tx.SenderAddress),
					MaxFee:            AdaptFelt(tx.MaxFee),
					Signature:         AdaptAccountSignature(tx.Signature()),
					ClassHash:         AdaptHash(tx.ClassHash),
					Nonce:             AdaptFelt(tx.Nonce),
					CompiledClassHash: AdaptFelt(tx.CompiledClassHash),
					L1Gas:             adaptResourceLimits(tx.ResourceBounds[core.ResourceL1Gas]),
					L2Gas:             adaptResourceLimits(tx.ResourceBounds[core.ResourceL2Gas]),
					Tip:               AdaptFelt(new(felt.Felt).SetUint64(tx.Tip)),
					// todo fill rest of V3 fields:
					Paymaster:   nil,
					NonceDomain: "",
					FeeDomain:   "",
				},
			}
		default:
			panic(fmt.Errorf("unsupported Declare transaction version %s", tx.Version))
		}
	case *core.InvokeTransaction:
		switch {
		case tx.Version.Is(0):
			specTx.Txn = &spec.Transaction_InvokeV0_{
				InvokeV0: &spec.Transaction_InvokeV0{
					MaxFee:             AdaptFelt(tx.MaxFee),
					Signature:          AdaptAccountSignature(tx.Signature()),
					Address:            AdaptAddress(tx.ContractAddress),
					EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
					Calldata:           AdaptFeltSlice(tx.CallData),
				},
			}
		case tx.Version.Is(1):
			specTx.Txn = &spec.Transaction_InvokeV1_{
				InvokeV1: &spec.Transaction_InvokeV1{
					Sender:    AdaptAddress(tx.SenderAddress),
					MaxFee:    AdaptFelt(tx.MaxFee),
					Signature: AdaptAccountSignature(tx.Signature()),
					Calldata:  AdaptFeltSlice(tx.CallData),
					Nonce:     AdaptFelt(tx.Nonce),
				},
			}
		case tx.Version.Is(3):
			specTx.Txn = &spec.Transaction_InvokeV3_{
				InvokeV3: &spec.Transaction_InvokeV3{
					Sender:    AdaptAddress(tx.SenderAddress),
					MaxFee:    AdaptFelt(tx.MaxFee),
					Signature: AdaptAccountSignature(tx.Signature()),
					Calldata:  AdaptFeltSlice(tx.CallData),
					Nonce:     AdaptFelt(tx.Nonce),
					L1Gas:     adaptResourceLimits(tx.ResourceBounds[core.ResourceL1Gas]),
					L2Gas:     adaptResourceLimits(tx.ResourceBounds[core.ResourceL2Gas]),
					Tip:       AdaptFelt(new(felt.Felt).SetUint64(tx.Tip)),
					// todo fill rest of V3 fields:
					Paymaster:   nil,
					NonceDomain: "",
					FeeDomain:   "",
				},
			}
		default:
			panic(fmt.Errorf("unsupported Invoke transaction version %s", tx.Version))
		}
	case *core.L1HandlerTransaction:
		specTx.Txn = adaptL1HandlerTransaction(tx)
	}

	return &specTx
}

func adaptResourceLimits(bounds core.ResourceBounds) *spec.ResourceLimits {
	maxAmount := new(felt.Felt).SetUint64(bounds.MaxAmount)
	return &spec.ResourceLimits{
		MaxAmount:       AdaptFelt(maxAmount),
		MaxPricePerUnit: AdaptFelt(bounds.MaxPricePerUnit),
	}
}

func adaptDeployTransaction(tx *core.DeployTransaction) *spec.Transaction_Deploy_ {
	return &spec.Transaction_Deploy_{
		Deploy: &spec.Transaction_Deploy{
			ClassHash:   AdaptHash(tx.ClassHash),
			AddressSalt: AdaptFelt(tx.ContractAddressSalt),
			Calldata:    AdaptFeltSlice(tx.ConstructorCallData),
		},
	}
}

func adaptL1HandlerTransaction(tx *core.L1HandlerTransaction) *spec.Transaction_L1Handler {
	return &spec.Transaction_L1Handler{
		L1Handler: &spec.Transaction_L1HandlerV1{
			Nonce:              AdaptFelt(tx.Nonce),
			Address:            AdaptAddress(tx.ContractAddress),
			EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
			Calldata:           AdaptFeltSlice(tx.CallData),
		},
	}
}

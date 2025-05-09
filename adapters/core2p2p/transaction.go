package core2p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/gen"
	"github.com/NethermindEth/juno/utils"
)

//nolint:funlen,gocyclo
func AdaptTransaction(transaction core.Transaction) *gen.Transaction {
	if transaction == nil {
		return nil
	}

	var specTx gen.Transaction

	switch tx := transaction.(type) {
	case *core.DeployTransaction:
		specTx.Txn = adaptDeployTransaction(tx)
	case *core.DeployAccountTransaction:
		switch {
		case tx.Version.Is(1):
			specTx.Txn = &gen.Transaction_DeployAccountV1_{
				DeployAccountV1: &gen.Transaction_DeployAccountV1{
					MaxFee:      AdaptFelt(tx.MaxFee),
					Signature:   AdaptAccountSignature(tx.Signature()),
					ClassHash:   AdaptHash(tx.ClassHash),
					Nonce:       AdaptFelt(tx.Nonce),
					AddressSalt: AdaptFelt(tx.ContractAddressSalt),
					Calldata:    AdaptFeltSlice(tx.ConstructorCallData),
				},
			}
		case tx.Version.Is(3):
			specTx.Txn = &gen.Transaction_DeployAccountV3_{
				DeployAccountV3: &gen.Transaction_DeployAccountV3{
					Signature:                 AdaptAccountSignature(tx.Signature()),
					ClassHash:                 AdaptHash(tx.ClassHash),
					Nonce:                     AdaptFelt(tx.Nonce),
					AddressSalt:               AdaptFelt(tx.ContractAddressSalt),
					Calldata:                  AdaptFeltSlice(tx.ConstructorCallData),
					ResourceBounds:            adaptResourceBounds(tx.ResourceBounds),
					Tip:                       tx.Tip,
					PaymasterData:             utils.Map(tx.PaymasterData, AdaptFelt),
					NonceDataAvailabilityMode: adaptVolitionDomain(tx.NonceDAMode),
					FeeDataAvailabilityMode:   adaptVolitionDomain(tx.FeeDAMode),
				},
			}
		default:
			panic(fmt.Errorf("unsupported InvokeTransaction version %s", tx.Version))
		}
	case *core.DeclareTransaction:
		switch {
		case tx.Version.Is(0):
			specTx.Txn = &gen.Transaction_DeclareV0_{
				DeclareV0: &gen.Transaction_DeclareV0{
					Sender:    AdaptAddress(tx.SenderAddress),
					MaxFee:    AdaptFelt(tx.MaxFee),
					Signature: AdaptAccountSignature(tx.Signature()),
					ClassHash: AdaptHash(tx.ClassHash),
				},
			}
		case tx.Version.Is(1):
			specTx.Txn = &gen.Transaction_DeclareV1_{
				DeclareV1: &gen.Transaction_DeclareV1{
					Sender:    AdaptAddress(tx.SenderAddress),
					MaxFee:    AdaptFelt(tx.MaxFee),
					Signature: AdaptAccountSignature(tx.Signature()),
					ClassHash: AdaptHash(tx.ClassHash),
					Nonce:     AdaptFelt(tx.Nonce),
				},
			}
		case tx.Version.Is(2):
			specTx.Txn = &gen.Transaction_DeclareV2_{
				DeclareV2: &gen.Transaction_DeclareV2{
					Sender:            AdaptAddress(tx.SenderAddress),
					MaxFee:            AdaptFelt(tx.MaxFee),
					Signature:         AdaptAccountSignature(tx.Signature()),
					ClassHash:         AdaptHash(tx.ClassHash),
					Nonce:             AdaptFelt(tx.Nonce),
					CompiledClassHash: AdaptHash(tx.CompiledClassHash),
				},
			}
		case tx.Version.Is(3):
			specTx.Txn = &gen.Transaction_DeclareV3_{
				DeclareV3: &gen.Transaction_DeclareV3{
					Common: &gen.DeclareV3Common{
						Sender:                    AdaptAddress(tx.SenderAddress),
						Signature:                 AdaptAccountSignature(tx.Signature()),
						Nonce:                     AdaptFelt(tx.Nonce),
						CompiledClassHash:         AdaptHash(tx.CompiledClassHash),
						ResourceBounds:            adaptResourceBounds(tx.ResourceBounds),
						Tip:                       tx.Tip,
						PaymasterData:             utils.Map(tx.PaymasterData, AdaptFelt),
						AccountDeploymentData:     utils.Map(tx.AccountDeploymentData, AdaptFelt),
						NonceDataAvailabilityMode: adaptVolitionDomain(tx.NonceDAMode),
						FeeDataAvailabilityMode:   adaptVolitionDomain(tx.FeeDAMode),
					},
					ClassHash: AdaptHash(tx.ClassHash),
				},
			}
		default:
			panic(fmt.Errorf("unsupported Declare transaction version %s", tx.Version))
		}
	case *core.InvokeTransaction:
		switch {
		case tx.Version.Is(0):
			specTx.Txn = &gen.Transaction_InvokeV0_{
				InvokeV0: &gen.Transaction_InvokeV0{
					MaxFee:             AdaptFelt(tx.MaxFee),
					Signature:          AdaptAccountSignature(tx.Signature()),
					Address:            AdaptAddress(tx.ContractAddress),
					EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
					Calldata:           AdaptFeltSlice(tx.CallData),
				},
			}
		case tx.Version.Is(1):
			specTx.Txn = &gen.Transaction_InvokeV1_{
				InvokeV1: &gen.Transaction_InvokeV1{
					Sender:    AdaptAddress(tx.SenderAddress),
					MaxFee:    AdaptFelt(tx.MaxFee),
					Signature: AdaptAccountSignature(tx.Signature()),
					Calldata:  AdaptFeltSlice(tx.CallData),
					Nonce:     AdaptFelt(tx.Nonce),
				},
			}
		case tx.Version.Is(3):
			specTx.Txn = &gen.Transaction_InvokeV3_{
				InvokeV3: &gen.Transaction_InvokeV3{
					Sender:                    AdaptAddress(tx.SenderAddress),
					Signature:                 AdaptAccountSignature(tx.Signature()),
					Calldata:                  AdaptFeltSlice(tx.CallData),
					ResourceBounds:            adaptResourceBounds(tx.ResourceBounds),
					Tip:                       tx.Tip,
					PaymasterData:             utils.Map(tx.PaymasterData, AdaptFelt),
					AccountDeploymentData:     utils.Map(tx.AccountDeploymentData, AdaptFelt),
					NonceDataAvailabilityMode: adaptVolitionDomain(tx.NonceDAMode),
					FeeDataAvailabilityMode:   adaptVolitionDomain(tx.FeeDAMode),
					Nonce:                     AdaptFelt(tx.Nonce),
				},
			}
		default:
			panic(fmt.Errorf("unsupported Invoke transaction version %s", tx.Version))
		}
	case *core.L1HandlerTransaction:
		specTx.Txn = adaptL1HandlerTransaction(tx)
	}

	specTx.TransactionHash = AdaptHash(transaction.Hash())

	return &specTx
}

func adaptResourceLimits(bounds core.ResourceBounds) *gen.ResourceLimits {
	maxAmount := new(felt.Felt).SetUint64(bounds.MaxAmount)
	return &gen.ResourceLimits{
		MaxAmount:       AdaptFelt(maxAmount),
		MaxPricePerUnit: AdaptFelt(bounds.MaxPricePerUnit),
	}
}

func adaptResourceBounds(rb map[core.Resource]core.ResourceBounds) *gen.ResourceBounds {
	return &gen.ResourceBounds{
		L1Gas:     adaptResourceLimits(rb[core.ResourceL1Gas]),
		L1DataGas: adaptResourceLimits(rb[core.ResourceL1DataGas]),
		L2Gas:     adaptResourceLimits(rb[core.ResourceL2Gas]),
	}
}

func adaptDeployTransaction(tx *core.DeployTransaction) *gen.Transaction_Deploy_ {
	return &gen.Transaction_Deploy_{
		Deploy: &gen.Transaction_Deploy{
			ClassHash:   AdaptHash(tx.ClassHash),
			AddressSalt: AdaptFelt(tx.ContractAddressSalt),
			Calldata:    AdaptFeltSlice(tx.ConstructorCallData),
			Version:     0, // todo(kirill) remove field from spec? tx is deprecated so no future versions
		},
	}
}

func adaptL1HandlerTransaction(tx *core.L1HandlerTransaction) *gen.Transaction_L1Handler {
	return &gen.Transaction_L1Handler{
		L1Handler: &gen.Transaction_L1HandlerV0{
			Nonce:              AdaptFelt(tx.Nonce),
			Address:            AdaptAddress(tx.ContractAddress),
			EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
			Calldata:           AdaptFeltSlice(tx.CallData),
		},
	}
}

func adaptVolitionDomain(mode core.DataAvailabilityMode) gen.VolitionDomain {
	switch mode {
	case core.DAModeL1:
		return gen.VolitionDomain_L1
	case core.DAModeL2:
		return gen.VolitionDomain_L2
	default:
		panic("unreachable in adaptVolitionDomain")
	}
}

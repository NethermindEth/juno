package core2p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	synctransaction "github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/transaction"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/transaction"
)

//nolint:gocyclo
func AdaptTransaction(tx core.Transaction) *synctransaction.TransactionInBlock {
	if tx == nil {
		return nil
	}

	var specTx synctransaction.TransactionInBlock

	switch tx := tx.(type) {
	case *core.DeployTransaction:
		specTx.Txn = adaptDeployTransaction(tx)
	case *core.DeployAccountTransaction:
		switch {
		case tx.Version.Is(1):
			specTx.Txn = &synctransaction.TransactionInBlock_DeployAccountV1_{
				DeployAccountV1: &synctransaction.TransactionInBlock_DeployAccountV1{
					MaxFee:      AdaptFelt(tx.MaxFee),
					Signature:   AdaptAccountSignature(tx.Signature()),
					ClassHash:   AdaptHash(tx.ClassHash),
					Nonce:       AdaptFelt(tx.Nonce),
					AddressSalt: AdaptFelt(tx.ContractAddressSalt),
					Calldata:    AdaptFeltSlice(tx.ConstructorCallData),
				},
			}
		case tx.Version.Is(3):
			specTx.Txn = &synctransaction.TransactionInBlock_DeployAccountV3{
				DeployAccountV3: AdaptDeployAccountV3Transaction(tx),
			}
		default:
			panic(fmt.Errorf("unsupported InvokeTransaction version %s", tx.Version))
		}
	case *core.DeclareTransaction:
		switch {
		case tx.Version.Is(0):
			specTx.Txn = &synctransaction.TransactionInBlock_DeclareV0{
				DeclareV0: &synctransaction.TransactionInBlock_DeclareV0WithoutClass{
					Sender:    AdaptAddress(tx.SenderAddress),
					MaxFee:    AdaptFelt(tx.MaxFee),
					Signature: AdaptAccountSignature(tx.Signature()),
					ClassHash: AdaptHash(tx.ClassHash),
				},
			}
		case tx.Version.Is(1):
			specTx.Txn = &synctransaction.TransactionInBlock_DeclareV1{
				DeclareV1: &synctransaction.TransactionInBlock_DeclareV1WithoutClass{
					Sender:    AdaptAddress(tx.SenderAddress),
					MaxFee:    AdaptFelt(tx.MaxFee),
					Signature: AdaptAccountSignature(tx.Signature()),
					ClassHash: AdaptHash(tx.ClassHash),
					Nonce:     AdaptFelt(tx.Nonce),
				},
			}
		case tx.Version.Is(2):
			specTx.Txn = &synctransaction.TransactionInBlock_DeclareV2{
				DeclareV2: &synctransaction.TransactionInBlock_DeclareV2WithoutClass{
					Sender:            AdaptAddress(tx.SenderAddress),
					MaxFee:            AdaptFelt(tx.MaxFee),
					Signature:         AdaptAccountSignature(tx.Signature()),
					ClassHash:         AdaptHash(tx.ClassHash),
					Nonce:             AdaptFelt(tx.Nonce),
					CompiledClassHash: AdaptHash(tx.CompiledClassHash),
				},
			}
		case tx.Version.Is(3):
			specTx.Txn = &synctransaction.TransactionInBlock_DeclareV3{
				DeclareV3: &synctransaction.TransactionInBlock_DeclareV3WithoutClass{
					Common:    AdaptDeclareV3Common(tx),
					ClassHash: AdaptHash(tx.ClassHash),
				},
			}
		default:
			panic(fmt.Errorf("unsupported Declare transaction version %s", tx.Version))
		}
	case *core.InvokeTransaction:
		switch {
		case tx.Version.Is(0):
			specTx.Txn = &synctransaction.TransactionInBlock_InvokeV0_{
				InvokeV0: &synctransaction.TransactionInBlock_InvokeV0{
					MaxFee:             AdaptFelt(tx.MaxFee),
					Signature:          AdaptAccountSignature(tx.Signature()),
					Address:            AdaptAddress(tx.ContractAddress),
					EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
					Calldata:           AdaptFeltSlice(tx.CallData),
				},
			}
		case tx.Version.Is(1):
			specTx.Txn = &synctransaction.TransactionInBlock_InvokeV1_{
				InvokeV1: &synctransaction.TransactionInBlock_InvokeV1{
					Sender:    AdaptAddress(tx.SenderAddress),
					MaxFee:    AdaptFelt(tx.MaxFee),
					Signature: AdaptAccountSignature(tx.Signature()),
					Calldata:  AdaptFeltSlice(tx.CallData),
					Nonce:     AdaptFelt(tx.Nonce),
				},
			}
		case tx.Version.Is(3):
			specTx.Txn = &synctransaction.TransactionInBlock_InvokeV3{
				InvokeV3: AdaptInvokeV3Transaction(tx),
			}
		default:
			panic(fmt.Errorf("unsupported Invoke transaction version %s", tx.Version))
		}
	case *core.L1HandlerTransaction:
		specTx.Txn = &synctransaction.TransactionInBlock_L1Handler{
			L1Handler: AdaptL1HandlerTransaction(tx),
		}
	}

	specTx.TransactionHash = AdaptHash(tx.Hash())

	return &specTx
}

func adaptResourceLimits(bounds core.ResourceBounds) *transaction.ResourceLimits {
	maxAmount := new(felt.Felt).SetUint64(bounds.MaxAmount)
	return &transaction.ResourceLimits{
		MaxAmount:       AdaptFelt(maxAmount),
		MaxPricePerUnit: AdaptFelt(bounds.MaxPricePerUnit),
	}
}

func adaptResourceBounds(rb map[core.Resource]core.ResourceBounds) *transaction.ResourceBounds {
	return &transaction.ResourceBounds{
		L1Gas:     adaptResourceLimits(rb[core.ResourceL1Gas]),
		L1DataGas: adaptResourceLimits(rb[core.ResourceL1DataGas]),
		L2Gas:     adaptResourceLimits(rb[core.ResourceL2Gas]),
	}
}

func adaptDeployTransaction(tx *core.DeployTransaction) *synctransaction.TransactionInBlock_Deploy_ {
	return &synctransaction.TransactionInBlock_Deploy_{
		Deploy: &synctransaction.TransactionInBlock_Deploy{
			ClassHash:   AdaptHash(tx.ClassHash),
			AddressSalt: AdaptFelt(tx.ContractAddressSalt),
			Calldata:    AdaptFeltSlice(tx.ConstructorCallData),
			Version:     0, // todo(kirill) remove field from spec? tx is deprecated so no future versions
		},
	}
}

func AdaptDeclareV3Common(tx *core.DeclareTransaction) *transaction.DeclareV3Common {
	return &transaction.DeclareV3Common{
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
	}
}

func AdaptDeployAccountV3Transaction(tx *core.DeployAccountTransaction) *transaction.DeployAccountV3 {
	return &transaction.DeployAccountV3{
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
	}
}

func AdaptInvokeV3Transaction(tx *core.InvokeTransaction) *transaction.InvokeV3 {
	return &transaction.InvokeV3{
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
	}
}

func AdaptL1HandlerTransaction(tx *core.L1HandlerTransaction) *transaction.L1HandlerV0 {
	return &transaction.L1HandlerV0{
		Nonce:              AdaptFelt(tx.Nonce),
		Address:            AdaptAddress(tx.ContractAddress),
		EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
		Calldata:           AdaptFeltSlice(tx.CallData),
	}
}

func adaptVolitionDomain(mode core.DataAvailabilityMode) common.VolitionDomain {
	switch mode {
	case core.DAModeL1:
		return common.VolitionDomain_L1
	case core.DAModeL2:
		return common.VolitionDomain_L2
	default:
		panic("unreachable in adaptVolitionDomain")
	}
}

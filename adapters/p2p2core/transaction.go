package p2p2core

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	synctransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/transaction"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
)

func AdaptDeclareV3TxnCommon(tx *transaction.DeclareV3Common, classHash, txnHash *common.Hash) *core.DeclareTransaction {
	nDAMode, err := adaptVolitionDomain(tx.NonceDataAvailabilityMode)
	if err != nil {
		panic(fmt.Sprintf("Failed to convert Nonce DA mode: %v to uint32", tx.NonceDataAvailabilityMode))
	}
	fDAMode, err := adaptVolitionDomain(tx.FeeDataAvailabilityMode)
	if err != nil {
		panic(fmt.Sprintf("Failed to convert Fee DA mode: %v to uint32", tx.FeeDataAvailabilityMode))
	}
	declareTx := &core.DeclareTransaction{
		TransactionHash:      AdaptHash(txnHash),
		ClassHash:            AdaptHash(classHash),
		SenderAddress:        AdaptAddress(tx.Sender),
		MaxFee:               nil, // in 3 version this field was removed
		TransactionSignature: adaptAccountSignature(tx.Signature),
		Nonce:                AdaptFelt(tx.Nonce),
		Version:              txVersion(3),
		CompiledClassHash:    AdaptHash(tx.CompiledClassHash),
		Tip:                  tx.Tip,
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas:     adaptResourceLimits(tx.ResourceBounds.L1Gas),
			core.ResourceL2Gas:     adaptResourceLimits(tx.ResourceBounds.L2Gas),
			core.ResourceL1DataGas: adaptResourceLimits(tx.ResourceBounds.L1DataGas),
		},
		PaymasterData:         utils.Map(tx.PaymasterData, AdaptFelt),
		AccountDeploymentData: utils.Map(tx.AccountDeploymentData, AdaptFelt),
		NonceDAMode:           nDAMode,
		FeeDAMode:             fDAMode,
	}
	return declareTx
}

func AdaptDeployAccountV3TxnCommon(tx *transaction.DeployAccountV3, txnHash *common.Hash) *core.DeployAccountTransaction {
	nDAMode, err := adaptVolitionDomain(tx.NonceDataAvailabilityMode)
	if err != nil {
		panic(fmt.Sprintf("Failed to convert Nonce DA mode: %v to uint32", tx.NonceDataAvailabilityMode))
	}

	fDAMode, err := adaptVolitionDomain(tx.FeeDataAvailabilityMode)
	if err != nil {
		panic(fmt.Sprintf("Failed to convert Fee DA mode: %v to uint32", tx.FeeDataAvailabilityMode))
	}

	addressSalt := AdaptFelt(tx.AddressSalt)
	classHash := AdaptHash(tx.ClassHash)
	callData := utils.Map(tx.Calldata, AdaptFelt)

	return &core.DeployAccountTransaction{
		DeployTransaction: core.DeployTransaction{
			TransactionHash:     AdaptHash(txnHash),
			ContractAddressSalt: addressSalt,
			ContractAddress:     core.ContractAddress(&felt.Zero, classHash, addressSalt, callData),
			ClassHash:           classHash,
			ConstructorCallData: callData,
			Version:             txVersion(3),
		},
		MaxFee:               nil, // todo(kirill) update spec? missing field
		TransactionSignature: adaptAccountSignature(tx.Signature),
		Nonce:                AdaptFelt(tx.Nonce),
		Tip:                  tx.Tip,
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas:     adaptResourceLimits(tx.ResourceBounds.L1Gas),
			core.ResourceL2Gas:     adaptResourceLimits(tx.ResourceBounds.L2Gas),
			core.ResourceL1DataGas: adaptResourceLimits(tx.ResourceBounds.L1DataGas),
		},
		PaymasterData: utils.Map(tx.PaymasterData, AdaptFelt),
		NonceDAMode:   nDAMode,
		FeeDAMode:     fDAMode,
	}
}

func AdaptInvokeV3TxnCommon(tx *transaction.InvokeV3, txnHash *common.Hash) *core.InvokeTransaction {
	nDAMode, err := adaptVolitionDomain(tx.NonceDataAvailabilityMode)
	if err != nil {
		panic(fmt.Sprintf("Failed to convert Nonce DA mode: %v to uint32", tx.NonceDataAvailabilityMode))
	}

	fDAMode, err := adaptVolitionDomain(tx.FeeDataAvailabilityMode)
	if err != nil {
		panic(fmt.Sprintf("Failed to convert Fee DA mode: %v to uint32", tx.FeeDataAvailabilityMode))
	}

	paymasterData := utils.Map(tx.PaymasterData, AdaptFelt)
	if paymasterData == nil {
		paymasterData = []*felt.Felt{}
	}
	return &core.InvokeTransaction{
		TransactionHash:      AdaptHash(txnHash),
		ContractAddress:      nil, // todo call core.ContractAddress() ?
		CallData:             utils.Map(tx.Calldata, AdaptFelt),
		TransactionSignature: adaptAccountSignature(tx.Signature),
		MaxFee:               nil, // in 3 version this field was removed
		Version:              txVersion(3),
		Nonce:                AdaptFelt(tx.Nonce),
		SenderAddress:        AdaptAddress(tx.Sender),
		EntryPointSelector:   nil,
		Tip:                  tx.Tip,
		ResourceBounds: map[core.Resource]core.ResourceBounds{
			core.ResourceL1Gas:     adaptResourceLimits(tx.ResourceBounds.L1Gas),
			core.ResourceL2Gas:     adaptResourceLimits(tx.ResourceBounds.L2Gas),
			core.ResourceL1DataGas: adaptResourceLimits(tx.ResourceBounds.L1DataGas),
		},
		PaymasterData:         paymasterData,
		NonceDAMode:           nDAMode,
		FeeDAMode:             fDAMode,
		AccountDeploymentData: []*felt.Felt{},
	}
}

func AdaptL1Handler(tx *transaction.L1HandlerV0, txnHash *common.Hash) *core.L1HandlerTransaction {
	return &core.L1HandlerTransaction{
		TransactionHash:    AdaptHash(txnHash),
		ContractAddress:    AdaptAddress(tx.Address),
		EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
		Nonce:              AdaptFelt(tx.Nonce),
		CallData:           utils.Map(tx.Calldata, AdaptFelt),
		Version:            txVersion(0),
	}
}

//nolint:funlen
func AdaptTransaction(t *synctransaction.TransactionInBlock, network *utils.Network) core.Transaction {
	if t == nil {
		return nil
	}

	// can Txn be nil?
	switch t.Txn.(type) {
	case *synctransaction.TransactionInBlock_DeclareV0:
		tx := t.GetDeclareV0()
		declareTx := &core.DeclareTransaction{
			TransactionHash:      AdaptHash(t.TransactionHash),
			Nonce:                nil, // for v0 nonce is not used for hash calculation
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Version:              txVersion(0),
			// version 2 field
			CompiledClassHash: nil,
			// version 3 fields (zero values)
			ResourceBounds:        nil,
			PaymasterData:         nil,
			AccountDeploymentData: nil,
			Tip:                   0,
			NonceDAMode:           0,
			FeeDAMode:             0,
		}

		return declareTx
	case *synctransaction.TransactionInBlock_DeclareV1:
		tx := t.GetDeclareV1()
		declareTx := &core.DeclareTransaction{
			TransactionHash:      AdaptHash(t.TransactionHash),
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
			Version:              txVersion(1),
			// version 2 field
			CompiledClassHash: nil,
			// version 3 fields (zero values)
			ResourceBounds:        nil,
			PaymasterData:         nil,
			AccountDeploymentData: nil,
			Tip:                   0,
			NonceDAMode:           0,
			FeeDAMode:             0,
		}

		return declareTx
	case *synctransaction.TransactionInBlock_DeclareV2:
		tx := t.GetDeclareV2()
		declareTx := &core.DeclareTransaction{
			TransactionHash:      AdaptHash(t.TransactionHash),
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
			Version:              txVersion(2),
			CompiledClassHash:    AdaptHash(tx.CompiledClassHash),
			// version 3 fields (zero values)
			ResourceBounds:        nil,
			PaymasterData:         nil,
			AccountDeploymentData: nil,
			Tip:                   0,
			NonceDAMode:           0,
			FeeDAMode:             0,
		}

		return declareTx
	case *synctransaction.TransactionInBlock_DeclareV3:
		tx := t.GetDeclareV3()
		return AdaptDeclareV3TxnCommon(tx.Common, tx.ClassHash, t.TransactionHash)
	case *synctransaction.TransactionInBlock_Deploy_:
		tx := t.GetDeploy()

		addressSalt := AdaptFelt(tx.AddressSalt)
		classHash := AdaptHash(tx.ClassHash)
		callData := utils.Map(tx.Calldata, AdaptFelt)
		deployTx := &core.DeployTransaction{
			TransactionHash:     AdaptHash(t.TransactionHash),
			ContractAddress:     core.ContractAddress(&felt.Zero, classHash, addressSalt, callData),
			ContractAddressSalt: addressSalt,
			ClassHash:           classHash,
			ConstructorCallData: callData,
			Version:             txVersion(0),
		}

		return deployTx
	case *synctransaction.TransactionInBlock_DeployAccountV1_:
		tx := t.GetDeployAccountV1()

		addressSalt := AdaptFelt(tx.AddressSalt)
		classHash := AdaptHash(tx.ClassHash)
		callData := utils.Map(tx.Calldata, AdaptFelt)
		deployAccTx := &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     AdaptHash(t.TransactionHash),
				ContractAddressSalt: addressSalt,
				ContractAddress:     core.ContractAddress(&felt.Zero, classHash, addressSalt, callData),
				ClassHash:           classHash,
				ConstructorCallData: callData,
				Version:             txVersion(1),
			},
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
			// version 3 fields (zero values)
			ResourceBounds: nil,
			PaymasterData:  nil,
			Tip:            0,
			NonceDAMode:    0,
			FeeDAMode:      0,
		}

		return deployAccTx
	case *synctransaction.TransactionInBlock_DeployAccountV3:
		tx := t.GetDeployAccountV3()
		return AdaptDeployAccountV3TxnCommon(tx, t.TransactionHash)
	case *synctransaction.TransactionInBlock_InvokeV0_:
		tx := t.GetInvokeV0()
		invTx := &core.InvokeTransaction{
			TransactionHash:      AdaptHash(t.TransactionHash),
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               AdaptFelt(tx.MaxFee),
			ContractAddress:      AdaptAddress(tx.Address),
			Version:              txVersion(0),
			EntryPointSelector:   AdaptFelt(tx.EntryPointSelector),
			// version 1 fields (zero values)
			Nonce:         nil,
			SenderAddress: nil,
			// version 3 fields (zero values)
			ResourceBounds:        nil,
			Tip:                   0,
			PaymasterData:         nil,
			AccountDeploymentData: nil,
			NonceDAMode:           0,
			FeeDAMode:             0,
		}

		return invTx
	case *synctransaction.TransactionInBlock_InvokeV1_:
		tx := t.GetInvokeV1()
		invTx := &core.InvokeTransaction{
			TransactionHash:      AdaptHash(t.TransactionHash),
			ContractAddress:      nil, // todo call core.ContractAddress() ?
			Nonce:                AdaptFelt(tx.Nonce),
			SenderAddress:        AdaptAddress(tx.Sender),
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               AdaptFelt(tx.MaxFee),
			Version:              txVersion(1),
			EntryPointSelector:   nil,
			// version 3 fields (zero values)
			ResourceBounds:        nil,
			Tip:                   0,
			PaymasterData:         nil,
			AccountDeploymentData: nil,
			NonceDAMode:           0,
			FeeDAMode:             0,
		}

		return invTx
	case *synctransaction.TransactionInBlock_InvokeV3:
		tx := t.GetInvokeV3()
		return AdaptInvokeV3TxnCommon(tx, t.TransactionHash)
	case *synctransaction.TransactionInBlock_L1Handler:
		tx := t.GetL1Handler()
		return AdaptL1Handler(tx, t.TransactionHash)
	default:
		panic(fmt.Errorf("unsupported tx type %T", t.Txn))
	}
}

func adaptResourceLimits(limits *transaction.ResourceLimits) core.ResourceBounds {
	return core.ResourceBounds{
		MaxAmount:       AdaptFelt(limits.MaxAmount).Uint64(),
		MaxPricePerUnit: AdaptFelt(limits.MaxPricePerUnit),
	}
}

func adaptAccountSignature(s *transaction.AccountSignature) []*felt.Felt {
	return utils.Map(s.Parts, AdaptFelt)
}

func txVersion(v uint64) *core.TransactionVersion {
	return new(core.TransactionVersion).SetUint64(v)
}

func adaptVolitionDomain(v common.VolitionDomain) (core.DataAvailabilityMode, error) {
	switch v {
	case common.VolitionDomain_L1:
		return core.DAModeL1, nil
	case common.VolitionDomain_L2:
		return core.DAModeL2, nil
	default:
		return 0, fmt.Errorf("unknown volition domain %d", v)
	}
}

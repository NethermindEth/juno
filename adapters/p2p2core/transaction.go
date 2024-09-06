package p2p2core

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

//nolint:funlen,gocyclo
func AdaptTransaction(t *spec.Transaction, network *utils.Network) core.Transaction {
	if t == nil {
		return nil
	}

	// can Txn be nil?
	switch t.Txn.(type) {
	case *spec.Transaction_DeclareV0_:
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
	case *spec.Transaction_DeclareV1_:
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
	case *spec.Transaction_DeclareV2_:
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
	case *spec.Transaction_DeclareV3_:
		tx := t.GetDeclareV3()

		nDAMode, err := adaptVolitionDomain(tx.NonceDataAvailabilityMode)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert Nonce DA mode: %v to uint32", tx.NonceDataAvailabilityMode))
		}

		fDAMode, err := adaptVolitionDomain(tx.FeeDataAvailabilityMode)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert Fee DA mode: %v to uint32", tx.FeeDataAvailabilityMode))
		}

		declareTx := &core.DeclareTransaction{
			TransactionHash:      AdaptHash(t.TransactionHash),
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               nil, // in 3 version this field was removed
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
			Version:              txVersion(3),
			CompiledClassHash:    AdaptHash(tx.CompiledClassHash),
			Tip:                  tx.Tip,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas: adaptResourceLimits(tx.ResourceBounds.L1Gas),
				core.ResourceL2Gas: adaptResourceLimits(tx.ResourceBounds.L2Gas),
			},
			PaymasterData:         utils.Map(tx.PaymasterData, AdaptFelt),
			AccountDeploymentData: utils.Map(tx.AccountDeploymentData, AdaptFelt),
			NonceDAMode:           nDAMode,
			FeeDAMode:             fDAMode,
		}

		return declareTx
	case *spec.Transaction_Deploy_:
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
	case *spec.Transaction_DeployAccountV1_:
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
	case *spec.Transaction_DeployAccountV3_:
		tx := t.GetDeployAccountV3()

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
		deployAccTx := &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     AdaptHash(t.TransactionHash),
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
				core.ResourceL1Gas: adaptResourceLimits(tx.ResourceBounds.L1Gas),
				core.ResourceL2Gas: adaptResourceLimits(tx.ResourceBounds.L2Gas),
			},
			PaymasterData: utils.Map(tx.PaymasterData, AdaptFelt),
			NonceDAMode:   nDAMode,
			FeeDAMode:     fDAMode,
		}

		return deployAccTx
	case *spec.Transaction_InvokeV0_:
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
	case *spec.Transaction_InvokeV1_:
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
	case *spec.Transaction_InvokeV3_:
		tx := t.GetInvokeV3()

		nDAMode, err := adaptVolitionDomain(tx.NonceDataAvailabilityMode)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert Nonce DA mode: %v to uint32", tx.NonceDataAvailabilityMode))
		}

		fDAMode, err := adaptVolitionDomain(tx.FeeDataAvailabilityMode)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert Fee DA mode: %v to uint32", tx.FeeDataAvailabilityMode))
		}

		invTx := &core.InvokeTransaction{
			TransactionHash:      AdaptHash(t.TransactionHash),
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
				core.ResourceL1Gas: adaptResourceLimits(tx.ResourceBounds.L1Gas),
				core.ResourceL2Gas: adaptResourceLimits(tx.ResourceBounds.L2Gas),
			},
			PaymasterData:         utils.Map(tx.PaymasterData, AdaptFelt),
			NonceDAMode:           nDAMode,
			FeeDAMode:             fDAMode,
			AccountDeploymentData: nil, // todo(kirill) recheck
		}

		return invTx
	case *spec.Transaction_L1Handler:
		tx := t.GetL1Handler()
		l1Tx := &core.L1HandlerTransaction{
			TransactionHash:    AdaptHash(t.TransactionHash),
			ContractAddress:    AdaptAddress(tx.Address),
			EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
			Nonce:              AdaptFelt(tx.Nonce),
			CallData:           utils.Map(tx.Calldata, AdaptFelt),
			Version:            txVersion(0),
		}

		return l1Tx
	default:
		panic(fmt.Errorf("unsupported tx type %T", t.Txn))
	}
}

func adaptResourceLimits(limits *spec.ResourceLimits) core.ResourceBounds {
	return core.ResourceBounds{
		MaxAmount:       AdaptFelt(limits.MaxAmount).Uint64(),
		MaxPricePerUnit: AdaptFelt(limits.MaxPricePerUnit),
	}
}

func adaptAccountSignature(s *spec.AccountSignature) []*felt.Felt {
	return utils.Map(s.Parts, AdaptFelt)
}

func txVersion(v uint64) *core.TransactionVersion {
	return new(core.TransactionVersion).SetUint64(v)
}

func adaptVolitionDomain(v spec.VolitionDomain) (core.DataAvailabilityMode, error) {
	switch v {
	case spec.VolitionDomain_L1:
		return core.DAModeL1, nil
	case spec.VolitionDomain_L2:
		return core.DAModeL2, nil
	default:
		return 0, fmt.Errorf("unknown volition domain %d", v)
	}
}

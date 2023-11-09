package p2p2core

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func AdaptTransaction(t *spec.Transaction) core.Transaction {
	if t == nil {
		return nil
	}

	// can Txn be nil?
	switch t.Txn.(type) {
	case *spec.Transaction_DeclareV0_:
		tx := t.GetDeclareV0()
		return &core.DeclareTransaction{
			TransactionHash:      nil, // todo where to get it?
			Nonce:                nil, // for v0 nonce is not used for hash calculation
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Version:              txVerion(0),
			CompiledClassHash:    nil,
		}
	case *spec.Transaction_DeclareV1_:
		tx := t.GetDeclareV1()
		return &core.DeclareTransaction{
			TransactionHash:      nil, // todo where to get it?
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
			Version:              txVerion(1),
			CompiledClassHash:    nil,
		}
	case *spec.Transaction_DeclareV2_:
		tx := t.GetDeclareV2()
		return &core.DeclareTransaction{
			TransactionHash:      nil, // todo where to get it?
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
			Version:              txVerion(2),
			CompiledClassHash:    AdaptFelt(tx.CompiledClassHash),
		}
	case *spec.Transaction_DeclareV3_:
		tx := t.GetDeclareV3()
		return &core.DeclareTransaction{
			TransactionHash:      nil, // todo where to get it?
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
			Version:              txVerion(2),
			CompiledClassHash:    AdaptFelt(tx.CompiledClassHash),
		}
	case *spec.Transaction_Deploy_:
		tx := t.GetDeploy()

		addressSalt := AdaptFelt(tx.AddressSalt)
		classHash := AdaptHash(tx.ClassHash)
		callData := utils.Map(tx.Calldata, AdaptFelt)
		return &core.DeployTransaction{
			TransactionHash:     nil, // todo compute?
			ContractAddress:     core.ContractAddress(&felt.Zero, classHash, addressSalt, callData),
			ContractAddressSalt: addressSalt,
			ClassHash:           classHash,
			ConstructorCallData: callData,
			Version:             txVerion(0),
		}
	case *spec.Transaction_DeployAccountV1_:
		tx := t.GetDeployAccountV1()

		addressSalt := AdaptFelt(tx.AddressSalt)
		classHash := AdaptHash(tx.ClassHash)
		callData := utils.Map(tx.Calldata, AdaptFelt)
		return &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     nil, // todo compute?
				ContractAddressSalt: addressSalt,
				ContractAddress:     core.ContractAddress(&felt.Zero, classHash, addressSalt, callData),
				ClassHash:           classHash,
				ConstructorCallData: callData,
				Version:             txVerion(1),
			},
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
		}
	case *spec.Transaction_DeployAccountV3_:
		tx := t.GetDeployAccountV3()

		addressSalt := AdaptFelt(tx.AddressSalt)
		classHash := AdaptHash(tx.ClassHash)
		callData := utils.Map(tx.Calldata, AdaptFelt)
		return &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     nil, // todo compute?
				ContractAddressSalt: addressSalt,
				ContractAddress:     core.ContractAddress(&felt.Zero, classHash, addressSalt, callData),
				ClassHash:           classHash,
				ConstructorCallData: callData,
				Version:             txVerion(3),
			},
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
		}
	case *spec.Transaction_InvokeV0_:
		tx := t.GetInvokeV0()
		return &core.InvokeTransaction{
			TransactionHash:      nil, // todo compute
			Nonce:                nil, // not used in v0
			SenderAddress:        nil, // not used in v0
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               AdaptFelt(tx.MaxFee),
			ContractAddress:      AdaptAddress(tx.Address),
			Version:              txVerion(0),
			EntryPointSelector:   AdaptFelt(tx.EntryPointSelector),
		}
	case *spec.Transaction_InvokeV1_:
		tx := t.GetInvokeV1()
		return &core.InvokeTransaction{
			TransactionHash:      nil,
			ContractAddress:      nil, // not used in v1
			Nonce:                AdaptFelt(tx.Nonce),
			SenderAddress:        AdaptAddress(tx.Sender),
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               AdaptFelt(tx.MaxFee),
			Version:              txVerion(1),
			EntryPointSelector:   nil,
		}
	case *spec.Transaction_InvokeV3_:
		tx := t.GetInvokeV3()
		return &core.InvokeTransaction{
			TransactionHash:      nil,
			ContractAddress:      nil, // is it ok?
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               AdaptFelt(tx.MaxFee),
			Version:              txVerion(3),
			Nonce:                AdaptFelt(tx.Nonce),
			SenderAddress:        AdaptAddress(tx.Sender),
			EntryPointSelector:   nil,
		}
	case *spec.Transaction_L1Handler:
		tx := t.GetL1Handler()
		return &core.L1HandlerTransaction{
			TransactionHash:    nil, // todo compute
			ContractAddress:    AdaptAddress(tx.Address),
			EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
			Nonce:              AdaptFelt(tx.Nonce),
			CallData:           utils.Map(tx.Calldata, AdaptFelt),
			Version:            txVerion(0),
		}
	default:
		panic(fmt.Errorf("unsupported tx type %T", t.Txn))
	}
}

func adaptAccountSignature(s *spec.AccountSignature) []*felt.Felt {
	return utils.Map(s.Parts, AdaptFelt)
}

func txVerion(v uint64) *core.TransactionVersion {
	return new(core.TransactionVersion).SetUint64(v)
}

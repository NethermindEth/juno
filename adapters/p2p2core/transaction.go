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
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                nil, // todo recheck
			Version:              txVerion(0),
			CompiledClassHash:    nil, // todo recheck
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
			CompiledClassHash:    nil, // todo recheck
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
		return &core.DeployTransaction{
			TransactionHash:     nil, // todo compute?
			ContractAddressSalt: AdaptFelt(tx.AddressSalt),
			ContractAddress:     nil, // todo what to use?
			ClassHash:           AdaptHash(tx.ClassHash),
			ConstructorCallData: utils.Map(tx.Calldata, AdaptFelt),
			Version:             txVerion(0), // todo right?
		}
	case *spec.Transaction_DeployAccountV1_:
		tx := t.GetDeployAccountV1()
		return &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     nil, // todo compute?
				ContractAddressSalt: AdaptFelt(tx.AddressSalt),
				ContractAddress:     nil, // todo where to use it?
				ClassHash:           AdaptHash(tx.ClassHash),
				ConstructorCallData: utils.Map(tx.Calldata, AdaptFelt),
				Version:             txVerion(1),
			},
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
		}
	case *spec.Transaction_DeployAccountV3_:
		tx := t.GetDeployAccountV3()
		return &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     nil, // todo compute?
				ContractAddressSalt: AdaptFelt(tx.AddressSalt),
				ContractAddress:     nil, // todo where to use it?
				ClassHash:           AdaptHash(tx.ClassHash),
				ConstructorCallData: utils.Map(tx.Calldata, AdaptFelt),
				Version:             txVerion(3),
			},
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
		}
	case *spec.Transaction_InvokeV0_:
		tx := t.GetInvokeV0()
		return &core.InvokeTransaction{
			TransactionHash:      nil,
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               AdaptFelt(tx.MaxFee),
			ContractAddress:      AdaptAddress(tx.Address),
			Version:              txVerion(0),
			EntryPointSelector:   AdaptFelt(tx.EntryPointSelector),
			Nonce:                nil, // todo not set?
			SenderAddress:        nil, // todo not set?
		}
	case *spec.Transaction_InvokeV1_:
		tx := t.GetInvokeV1()
		return &core.InvokeTransaction{
			TransactionHash:      nil,
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               AdaptFelt(tx.MaxFee),
			ContractAddress:      nil, // todo what to use here?
			Version:              txVerion(1),
			EntryPointSelector:   nil,                     // what to use here?
			Nonce:                nil,                     // todo not set?
			SenderAddress:        AdaptAddress(tx.Sender), // todo not set?
		}
	case *spec.Transaction_InvokeV3_:
		tx := t.GetInvokeV3()
		return &core.InvokeTransaction{
			TransactionHash:      nil,
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               AdaptFelt(tx.MaxFee),
			ContractAddress:      nil, // todo what to use here?
			Version:              txVerion(3),
			EntryPointSelector:   nil,                     // what to use here?
			Nonce:                nil,                     // todo not set? (nonce domain?)
			SenderAddress:        AdaptAddress(tx.Sender), // todo not set?
		}
	case *spec.Transaction_L1Handler:
		tx := t.GetL1Handler()
		return &core.L1HandlerTransaction{
			TransactionHash:    nil, // todo compute
			ContractAddress:    AdaptAddress(tx.Address),
			EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
			Nonce:              AdaptFelt(tx.Nonce),
			CallData:           utils.Map(tx.Calldata, AdaptFelt),
			Version:            txVerion(0), // ok?
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

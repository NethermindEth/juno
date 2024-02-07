package p2p2core

import (
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

//nolint:funlen
func AdaptTransaction(t *spec.Transaction, network *utils.Network) core.Transaction {
	if t == nil {
		return nil
	}

	hash := func(tx core.Transaction) *felt.Felt {
		h, err := core.TransactionHash(tx, network)
		if err != nil {
			panic(fmt.Errorf("failed to compute transaction hash: %w", err))
		}

		return h
	}

	// can Txn be nil?
	switch t.Txn.(type) {
	case *spec.Transaction_DeclareV0_:
		tx := t.GetDeclareV0()
		declareTx := &core.DeclareTransaction{
			Nonce:                nil, // for v0 nonce is not used for hash calculation
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Version:              txVerion(0),
			CompiledClassHash:    nil,
		}
		declareTx.TransactionHash = hash(declareTx)

		return declareTx
	case *spec.Transaction_DeclareV1_:
		tx := t.GetDeclareV1()
		declareTx := &core.DeclareTransaction{
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
			Version:              txVerion(1),
			CompiledClassHash:    nil,
		}
		declareTx.TransactionHash = hash(declareTx)

		return declareTx
	case *spec.Transaction_DeclareV2_:
		tx := t.GetDeclareV2()
		declareTx := &core.DeclareTransaction{
			TransactionHash:      nil, // todo where to get it?
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			MaxFee:               AdaptFelt(tx.MaxFee),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
			Version:              txVerion(2),
			CompiledClassHash:    AdaptFelt(tx.CompiledClassHash),
		}
		declareTx.TransactionHash = hash(declareTx)

		return declareTx
	case *spec.Transaction_DeclareV3_:
		tx := t.GetDeclareV3()
		declareTx := &core.DeclareTransaction{
			ClassHash:            AdaptHash(tx.ClassHash),
			SenderAddress:        AdaptAddress(tx.Sender),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
			Version:              txVerion(2),
			CompiledClassHash:    AdaptFelt(tx.CompiledClassHash),
		}
		declareTx.TransactionHash = hash(declareTx)

		return declareTx
	case *spec.Transaction_Deploy_:
		tx := t.GetDeploy()

		addressSalt := AdaptFelt(tx.AddressSalt)
		classHash := AdaptHash(tx.ClassHash)
		callData := utils.Map(tx.Calldata, AdaptFelt)
		deployTx := &core.DeployTransaction{
			ContractAddress:     core.ContractAddress(&felt.Zero, classHash, addressSalt, callData),
			ContractAddressSalt: addressSalt,
			ClassHash:           classHash,
			ConstructorCallData: callData,
			Version:             txVerion(0),
		}
		deployTx.TransactionHash = hash(deployTx)

		return deployTx
	case *spec.Transaction_DeployAccountV1_:
		tx := t.GetDeployAccountV1()

		addressSalt := AdaptFelt(tx.AddressSalt)
		classHash := AdaptHash(tx.ClassHash)
		callData := utils.Map(tx.Calldata, AdaptFelt)
		deployAccTx := &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
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
		deployAccTx.DeployTransaction.TransactionHash = hash(deployAccTx)

		return deployAccTx
	case *spec.Transaction_DeployAccountV3_:
		tx := t.GetDeployAccountV3()

		addressSalt := AdaptFelt(tx.AddressSalt)
		classHash := AdaptHash(tx.ClassHash)
		callData := utils.Map(tx.Calldata, AdaptFelt)
		deployAccTx := &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				ContractAddressSalt: addressSalt,
				ContractAddress:     core.ContractAddress(&felt.Zero, classHash, addressSalt, callData),
				ClassHash:           classHash,
				ConstructorCallData: callData,
				Version:             txVerion(3),
			},
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                AdaptFelt(tx.Nonce),
		}
		deployAccTx.DeployTransaction.TransactionHash = hash(deployAccTx)

		return deployAccTx
	case *spec.Transaction_InvokeV0_:
		tx := t.GetInvokeV0()
		invTx := &core.InvokeTransaction{
			Nonce:                nil, // not used in v0
			SenderAddress:        nil, // not used in v0
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               AdaptFelt(tx.MaxFee),
			ContractAddress:      AdaptAddress(tx.Address),
			Version:              txVerion(0),
			EntryPointSelector:   AdaptFelt(tx.EntryPointSelector),
		}
		invTx.TransactionHash = hash(invTx)

		return invTx
	case *spec.Transaction_InvokeV1_:
		tx := t.GetInvokeV1()
		invTx := &core.InvokeTransaction{
			ContractAddress:      nil, // not used in v1
			Nonce:                AdaptFelt(tx.Nonce),
			SenderAddress:        AdaptAddress(tx.Sender),
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               AdaptFelt(tx.MaxFee),
			Version:              txVerion(1),
			EntryPointSelector:   nil,
		}
		invTx.TransactionHash = hash(invTx)

		return invTx
	case *spec.Transaction_InvokeV3_:
		tx := t.GetInvokeV3()
		invTx := &core.InvokeTransaction{
			ContractAddress:      nil, // is it ok?
			CallData:             utils.Map(tx.Calldata, AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Version:              txVerion(3),
			Nonce:                AdaptFelt(tx.Nonce),
			SenderAddress:        AdaptAddress(tx.Sender),
			EntryPointSelector:   nil,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas: adaptResourceLimits(tx.L1Gas),
				core.ResourceL2Gas: adaptResourceLimits(tx.L2Gas),
			},
		}
		invTx.TransactionHash = hash(invTx)

		return invTx
	case *spec.Transaction_L1Handler:
		tx := t.GetL1Handler()
		l1Tx := &core.L1HandlerTransaction{
			ContractAddress:    AdaptAddress(tx.Address),
			EntryPointSelector: AdaptFelt(tx.EntryPointSelector),
			Nonce:              AdaptFelt(tx.Nonce),
			CallData:           utils.Map(tx.Calldata, AdaptFelt),
			Version:            txVerion(0),
		}
		l1Tx.TransactionHash = hash(l1Tx)

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

func txVerion(v uint64) *core.TransactionVersion {
	return new(core.TransactionVersion).SetUint64(v)
}

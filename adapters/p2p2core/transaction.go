package p2p2core

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	synctransaction "github.com/starknet-io/starknet-p2pspecs/p2p/proto/sync/transaction"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
)

func AdaptDeclareV3WithClass(
	ctx context.Context,
	compiler compiler.Compiler,
	tx *transaction.DeclareV3WithClass,
	txnHash *common.Hash,
) (*core.DeclareTransaction, *core.SierraClass, error) {
	class, err := AdaptSierraClass(ctx, compiler, tx.Class)
	if err != nil {
		return nil, nil, err
	}

	classHash, err := class.Hash()
	if err != nil {
		return nil, nil, err
	}

	declareCommon, err := AdaptDeclareV3TxnCommon(tx.Common, &classHash, txnHash)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to adapt declare v3 transaction common: %w", err)
	}
	casmHash := class.Compiled.Hash(core.HashVersionV1)
	if casmHash != *declareCommon.CompiledClassHash {
		err := fmt.Errorf("compiled class hash mismatch: expected %s, got %s",
			&casmHash,
			declareCommon.CompiledClassHash,
		)
		return nil, nil, err
	}

	return declareCommon, &class, nil
}

func AdaptDeclareV0TxnCommon(
	t *synctransaction.TransactionInBlock,
	tx *synctransaction.TransactionInBlock_DeclareV0WithoutClass,
) *core.DeclareTransaction {
	return &core.DeclareTransaction{
		TransactionHash:      AdaptHash(t.TransactionHash),
		ClassHash:            AdaptHash(tx.ClassHash),
		SenderAddress:        AdaptAddress(tx.Sender),
		MaxFee:               AdaptFelt(tx.MaxFee),
		TransactionSignature: adaptAccountSignature(tx.Signature),
		Nonce:                nil, // for v0 nonce is not used for hash calculation
		Version:              txVersion(0),
		// version 2 field
		CompiledClassHash: nil,
		// version 3 fields (zero values)
		ResourceBounds:        nil,
		Tip:                   0,
		PaymasterData:         nil,
		AccountDeploymentData: nil,
		NonceDAMode:           0,
		FeeDAMode:             0,
	}
}

func AdaptDeclareV1TxnCommon(
	t *synctransaction.TransactionInBlock,
	tx *synctransaction.TransactionInBlock_DeclareV1WithoutClass,
) *core.DeclareTransaction {
	return &core.DeclareTransaction{
		TransactionHash:       AdaptHash(t.TransactionHash),
		ClassHash:             AdaptHash(tx.ClassHash),
		SenderAddress:         AdaptAddress(tx.Sender),
		MaxFee:                AdaptFelt(tx.MaxFee),
		TransactionSignature:  adaptAccountSignature(tx.Signature),
		Nonce:                 AdaptFelt(tx.Nonce),
		Version:               txVersion(1),
		CompiledClassHash:     nil, // this field is not available on v1
		ResourceBounds:        nil, // this field is not available on v1
		Tip:                   0,   // this field is not available on v1
		PaymasterData:         nil, // this field is not available on v1
		AccountDeploymentData: nil, // this field is not available on v1
		NonceDAMode:           0,   // this field is not available on v1
		FeeDAMode:             0,   // this field is not available on v1
	}
}

func AdaptDeclareV2TxnCommon(
	t *synctransaction.TransactionInBlock,
	tx *synctransaction.TransactionInBlock_DeclareV2WithoutClass,
) *core.DeclareTransaction {
	return &core.DeclareTransaction{
		TransactionHash:       AdaptHash(t.TransactionHash),
		ClassHash:             AdaptHash(tx.ClassHash),
		SenderAddress:         AdaptAddress(tx.Sender),
		MaxFee:                AdaptFelt(tx.MaxFee),
		TransactionSignature:  adaptAccountSignature(tx.Signature),
		Nonce:                 AdaptFelt(tx.Nonce),
		Version:               txVersion(2),
		CompiledClassHash:     AdaptHash(tx.CompiledClassHash),
		ResourceBounds:        nil, // this field is not available on v2
		Tip:                   0,   // this field is not available on v2
		PaymasterData:         nil, // this field is not available on v2
		AccountDeploymentData: nil, // this field is not available on v2
		NonceDAMode:           0,   // this field is not available on v2
		FeeDAMode:             0,   // this field is not available on v2
	}
}

func AdaptDeclareV3TxnCommon(
	tx *transaction.DeclareV3Common,
	classHash *felt.Felt,
	txnHash *common.Hash,
) (*core.DeclareTransaction, error) {
	nDAMode, err := adaptVolitionDomain(tx.NonceDataAvailabilityMode)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to convert Nonce DA mode %v to uint32: %w", tx.NonceDataAvailabilityMode, err,
			)
	}
	fDAMode, err := adaptVolitionDomain(tx.FeeDataAvailabilityMode)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to convert Fee DA mode %v to uint32: %w", tx.FeeDataAvailabilityMode, err,
			)
	}
	declareTx := &core.DeclareTransaction{
		TransactionHash:      AdaptHash(txnHash),
		ClassHash:            classHash,
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
	return declareTx, nil
}

func AdaptDeployTxnCommon(
	t *synctransaction.TransactionInBlock,
	tx *synctransaction.TransactionInBlock_Deploy,
) *core.DeployTransaction {
	addressSalt := AdaptFelt(tx.AddressSalt)
	classHash := AdaptHash(tx.ClassHash)
	callData := utils.Map(tx.Calldata, AdaptFelt)
	contractAddress := core.ContractAddress(&felt.Zero, classHash, addressSalt, callData)
	return &core.DeployTransaction{
		TransactionHash:     AdaptHash(t.TransactionHash),
		ContractAddress:     &contractAddress,
		ContractAddressSalt: addressSalt,
		ClassHash:           classHash,
		ConstructorCallData: callData,
		Version:             txVersion(0),
	}
}

func AdaptDeployAccountV1TxnCommon(
	t *synctransaction.TransactionInBlock,
	tx *synctransaction.TransactionInBlock_DeployAccountV1,
) *core.DeployAccountTransaction {
	addressSalt := AdaptFelt(tx.AddressSalt)
	classHash := AdaptHash(tx.ClassHash)
	callData := utils.Map(tx.Calldata, AdaptFelt)
	contractAddress := core.ContractAddress(&felt.Zero, classHash, addressSalt, callData)
	return &core.DeployAccountTransaction{
		DeployTransaction: core.DeployTransaction{
			TransactionHash:     AdaptHash(t.TransactionHash),
			ContractAddressSalt: addressSalt,
			ContractAddress:     &contractAddress,
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
}

func AdaptDeployAccountV3TxnCommon(
	tx *transaction.DeployAccountV3,
	txnHash *common.Hash,
) (*core.DeployAccountTransaction, error) {
	nDAMode, err := adaptVolitionDomain(tx.NonceDataAvailabilityMode)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to convert Nonce DA mode %v to uint32: %w", tx.NonceDataAvailabilityMode, err,
			)
	}

	fDAMode, err := adaptVolitionDomain(tx.FeeDataAvailabilityMode)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to convert Fee DA mode %v to uint32: %w", tx.FeeDataAvailabilityMode, err,
			)
	}

	addressSalt := AdaptFelt(tx.AddressSalt)
	classHash := AdaptHash(tx.ClassHash)
	callData := utils.Map(tx.Calldata, AdaptFelt)
	contractAddress := core.ContractAddress(&felt.Zero, classHash, addressSalt, callData)
	return &core.DeployAccountTransaction{
		DeployTransaction: core.DeployTransaction{
			TransactionHash:     AdaptHash(txnHash),
			ContractAddressSalt: addressSalt,
			ContractAddress:     &contractAddress,
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
	}, nil
}

func AdaptInvokeV0TxnCommon(
	t *synctransaction.TransactionInBlock,
	tx *synctransaction.TransactionInBlock_InvokeV0,
) *core.InvokeTransaction {
	return &core.InvokeTransaction{
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
}

func AdaptInvokeV1TxnCommon(
	t *synctransaction.TransactionInBlock,
	tx *synctransaction.TransactionInBlock_InvokeV1,
) *core.InvokeTransaction {
	return &core.InvokeTransaction{
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
}

func AdaptInvokeV3TxnCommon(
	tx *transaction.InvokeV3,
	txnHash *common.Hash,
) (*core.InvokeTransaction, error) {
	nDAMode, err := adaptVolitionDomain(tx.NonceDataAvailabilityMode)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to convert Nonce DA mode %v to uint32: %w", tx.NonceDataAvailabilityMode, err,
			)
	}

	fDAMode, err := adaptVolitionDomain(tx.FeeDataAvailabilityMode)
	if err != nil {
		return nil,
			fmt.Errorf(
				"failed to convert Fee DA mode %v to uint32: %w", tx.FeeDataAvailabilityMode, err,
			)
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
		PaymasterData:         utils.Map(tx.PaymasterData, AdaptFelt),
		NonceDAMode:           nDAMode,
		FeeDAMode:             fDAMode,
		AccountDeploymentData: nil, // todo(kirill) recheck
		ProofFacts:            nil,
	}, nil
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

func AdaptTransaction(
	t *synctransaction.TransactionInBlock,
	network *utils.Network,
) (core.Transaction, error) {
	// can Txn be nil?

	switch t.Txn.(type) {
	case *synctransaction.TransactionInBlock_DeclareV0:
		return AdaptDeclareV0TxnCommon(t, t.GetDeclareV0()), nil
	case *synctransaction.TransactionInBlock_DeclareV1:
		return AdaptDeclareV1TxnCommon(t, t.GetDeclareV1()), nil
	case *synctransaction.TransactionInBlock_DeclareV2:
		return AdaptDeclareV2TxnCommon(t, t.GetDeclareV2()), nil
	case *synctransaction.TransactionInBlock_DeclareV3:
		tx := t.GetDeclareV3()
		return AdaptDeclareV3TxnCommon(tx.Common, AdaptHash(tx.ClassHash), t.TransactionHash)
	case *synctransaction.TransactionInBlock_Deploy_:
		return AdaptDeployTxnCommon(t, t.GetDeploy()), nil
	case *synctransaction.TransactionInBlock_DeployAccountV1_:
		return AdaptDeployAccountV1TxnCommon(t, t.GetDeployAccountV1()), nil
	case *synctransaction.TransactionInBlock_DeployAccountV3:
		tx := t.GetDeployAccountV3()
		return AdaptDeployAccountV3TxnCommon(tx, t.TransactionHash)
	case *synctransaction.TransactionInBlock_InvokeV0_:
		return AdaptInvokeV0TxnCommon(t, t.GetInvokeV0()), nil
	case *synctransaction.TransactionInBlock_InvokeV1_:
		return AdaptInvokeV1TxnCommon(t, t.GetInvokeV1()), nil
	case *synctransaction.TransactionInBlock_InvokeV3:
		tx := t.GetInvokeV3()
		return AdaptInvokeV3TxnCommon(tx, t.TransactionHash)
	case *synctransaction.TransactionInBlock_L1Handler:
		tx := t.GetL1Handler()
		return AdaptL1Handler(tx, t.TransactionHash), nil

	default:
		return nil, fmt.Errorf("unsupported tx type %T", t.Txn)
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

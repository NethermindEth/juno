package rpc2core

import (
	"cmp"
	"fmt"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils"
)

// Each per-type adapter rejects a transaction missing a field that the adapter or its hash
// dereferences. The hashes of version 0 declares and of deploys are taken as served.
func adaptTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
	network *networks.Network,
) (core.Transaction, error) {
	if transaction.Version == nil {
		return nil, missingFieldError("version")
	}

	switch transaction.Type {
	case rpcv10.TxnDeclare:
		return adaptDeclareTransaction(transaction, receipt, network)
	case rpcv10.TxnDeploy:
		deploy, err := adaptDeployTransaction(transaction, receipt)
		if err != nil {
			return nil, err
		}
		return &deploy, nil
	case rpcv10.TxnDeployAccount:
		return adaptDeployAccountTransaction(transaction, receipt, network)
	case rpcv10.TxnInvoke:
		return adaptInvokeTransaction(transaction, receipt, network)
	case rpcv10.TxnL1Handler:
		return adaptL1HandlerTransaction(transaction, receipt)
	default:
		return nil, fmt.Errorf("unknown type %s", transaction.Type)
	}
}

func adaptDeclareTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
	network *networks.Network,
) (core.Transaction, error) {
	version := (*core.TransactionVersion)(transaction.Version)
	signature := missing("signature", transaction.Signature)
	senderAddress := missing("sender_address", transaction.SenderAddress)
	classHash := missing("class_hash", transaction.ClassHash)
	compiledClassHash := missing("compiled_class_hash", transaction.CompiledClassHash)
	maxFee := missing("max_fee", transaction.MaxFee)
	nonce := missing("nonce", transaction.Nonce)
	resourceBounds := missingResourceBounds("resource_bounds", transaction.ResourceBounds)

	missingField := signature
	switch {
	case version.Is(0):
	case version.Is(1):
		missingField = cmp.Or(missingField, senderAddress, classHash, maxFee, nonce)
	case version.Is(2):
		missingField = cmp.Or(missingField, senderAddress, classHash, compiledClassHash, maxFee, nonce)
	case version.Is(3):
		missingField = cmp.Or(
			missingField,
			senderAddress,
			classHash,
			compiledClassHash,
			nonce,
			resourceBounds,
		)
	default:
		return nil, unknownVersionError(version)
	}
	if missingField != "" {
		return nil, missingFieldError(missingField)
	}

	declare := &core.DeclareTransaction{
		TransactionHash:       receipt.Hash,
		ClassHash:             transaction.ClassHash,
		SenderAddress:         transaction.SenderAddress,
		MaxFee:                transaction.MaxFee,
		TransactionSignature:  *transaction.Signature,
		Nonce:                 transaction.Nonce,
		Version:               version,
		CompiledClassHash:     transaction.CompiledClassHash,
		ResourceBounds:        adaptResourceBounds(transaction.ResourceBounds),
		Tip:                   feltToUint64(transaction.Tip),
		PaymasterData:         derefSlice(transaction.PaymasterData),
		AccountDeploymentData: derefSlice(transaction.AccountDeploymentData),
		NonceDAMode:           adaptDataAvailabilityMode(transaction.NonceDAMode),
		FeeDAMode:             adaptDataAvailabilityMode(transaction.FeeDAMode),
	}

	err := dropAbsentL1DataGasBounds(declare, declare.ResourceBounds, network)
	if err != nil {
		return nil, err
	}
	return declare, nil
}

func adaptDeployTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
) (core.DeployTransaction, error) {
	constructorCallData := missing("constructor_calldata", transaction.ConstructorCallData)
	contractAddress := missing("contract_address in its receipt", receipt.ContractAddress)

	missingField := cmp.Or(constructorCallData, contractAddress)
	if missingField != "" {
		return core.DeployTransaction{}, missingFieldError(missingField)
	}

	return core.DeployTransaction{
		TransactionHash:     receipt.Hash,
		ContractAddressSalt: transaction.ContractAddressSalt,
		ContractAddress:     receipt.ContractAddress,
		ClassHash:           transaction.ClassHash,
		ConstructorCallData: *transaction.ConstructorCallData,
		Version:             (*core.TransactionVersion)(transaction.Version),
	}, nil
}

func adaptDeployAccountTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
	network *networks.Network,
) (core.Transaction, error) {
	deploy, err := adaptDeployTransaction(transaction, receipt)
	if err != nil {
		return nil, err
	}

	version := (*core.TransactionVersion)(transaction.Version)
	signature := missing("signature", transaction.Signature)
	classHash := missing("class_hash", transaction.ClassHash)
	salt := missing("contract_address_salt", transaction.ContractAddressSalt)
	maxFee := missing("max_fee", transaction.MaxFee)
	nonce := missing("nonce", transaction.Nonce)
	resourceBounds := missingResourceBounds("resource_bounds", transaction.ResourceBounds)

	missingField := cmp.Or(signature, classHash, salt, nonce)
	switch {
	case version.Is(1):
		missingField = cmp.Or(missingField, maxFee)
	case version.Is(3):
		missingField = cmp.Or(missingField, resourceBounds)
	default:
		return nil, unknownVersionError(version)
	}
	if missingField != "" {
		return nil, missingFieldError(missingField)
	}

	deployAccount := &core.DeployAccountTransaction{
		DeployTransaction:    deploy,
		MaxFee:               transaction.MaxFee,
		TransactionSignature: *transaction.Signature,
		Nonce:                transaction.Nonce,
		ResourceBounds:       adaptResourceBounds(transaction.ResourceBounds),
		Tip:                  feltToUint64(transaction.Tip),
		PaymasterData:        derefSlice(transaction.PaymasterData),
		NonceDAMode:          adaptDataAvailabilityMode(transaction.NonceDAMode),
		FeeDAMode:            adaptDataAvailabilityMode(transaction.FeeDAMode),
	}

	err = dropAbsentL1DataGasBounds(deployAccount, deployAccount.ResourceBounds, network)
	if err != nil {
		return nil, err
	}
	return deployAccount, nil
}

func adaptInvokeTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
	network *networks.Network,
) (core.Transaction, error) {
	version := (*core.TransactionVersion)(transaction.Version)
	callData := missing("calldata", transaction.CallData)
	signature := missing("signature", transaction.Signature)
	contractAddress := missing("contract_address", transaction.ContractAddress)
	entryPointSelector := missing("entry_point_selector", transaction.EntryPointSelector)
	senderAddress := missing("sender_address", transaction.SenderAddress)
	maxFee := missing("max_fee", transaction.MaxFee)
	nonce := missing("nonce", transaction.Nonce)
	resourceBounds := missingResourceBounds("resource_bounds", transaction.ResourceBounds)

	missingField := cmp.Or(callData, signature)
	switch {
	case version.Is(0):
		missingField = cmp.Or(missingField, contractAddress, entryPointSelector, maxFee)
	case version.Is(1):
		missingField = cmp.Or(missingField, senderAddress, maxFee, nonce)
	case version.Is(3):
		missingField = cmp.Or(missingField, senderAddress, nonce, resourceBounds)
	default:
		return nil, unknownVersionError(version)
	}
	if missingField != "" {
		return nil, missingFieldError(missingField)
	}

	invoke := &core.InvokeTransaction{
		TransactionHash:       receipt.Hash,
		CallData:              *transaction.CallData,
		TransactionSignature:  *transaction.Signature,
		MaxFee:                transaction.MaxFee,
		ContractAddress:       transaction.ContractAddress,
		Version:               version,
		EntryPointSelector:    transaction.EntryPointSelector,
		Nonce:                 transaction.Nonce,
		SenderAddress:         transaction.SenderAddress,
		ResourceBounds:        adaptResourceBounds(transaction.ResourceBounds),
		Tip:                   feltToUint64(transaction.Tip),
		PaymasterData:         derefSlice(transaction.PaymasterData),
		AccountDeploymentData: derefSlice(transaction.AccountDeploymentData),
		NonceDAMode:           adaptDataAvailabilityMode(transaction.NonceDAMode),
		FeeDAMode:             adaptDataAvailabilityMode(transaction.FeeDAMode),
		ProofFacts:            utils.DerefSlice(transaction.ProofFacts),
	}

	err := dropAbsentL1DataGasBounds(invoke, invoke.ResourceBounds, network)
	if err != nil {
		return nil, err
	}
	return invoke, nil
}

func adaptL1HandlerTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
) (core.Transaction, error) {
	callData := missingOrEmpty("calldata", transaction.CallData)
	contractAddress := missing("contract_address", transaction.ContractAddress)
	entryPointSelector := missing("entry_point_selector", transaction.EntryPointSelector)

	missingField := cmp.Or(callData, contractAddress, entryPointSelector)
	if missingField != "" {
		return nil, missingFieldError(missingField)
	}

	return &core.L1HandlerTransaction{
		TransactionHash:    receipt.Hash,
		ContractAddress:    transaction.ContractAddress,
		EntryPointSelector: transaction.EntryPointSelector,
		Nonce:              transaction.Nonce,
		CallData:           *transaction.CallData,
		Version:            (*core.TransactionVersion)(transaction.Version),
	}, nil
}

func adaptResourceBounds(bounds *rpcv10.ResourceBoundsMap) map[core.Resource]core.ResourceBounds {
	if bounds == nil {
		return nil
	}
	return map[core.Resource]core.ResourceBounds{
		core.ResourceL1Gas:     adaptResourceBound(&bounds.L1Gas),
		core.ResourceL2Gas:     adaptResourceBound(&bounds.L2Gas),
		core.ResourceL1DataGas: adaptResourceBound(&bounds.L1DataGas),
	}
}

func adaptResourceBound(bound *rpcv10.ResourceBounds) core.ResourceBounds {
	return core.ResourceBounds{
		MaxAmount:       feltToUint64(bound.MaxAmount),
		MaxPricePerUnit: bound.MaxPricePerUnit,
	}
}

func adaptDataAvailabilityMode(mode *rpcv10.DataAvailabilityMode) core.DataAvailabilityMode {
	if mode == nil {
		return core.DAModeL1
	}
	return core.DataAvailabilityMode(*mode)
}

// dropAbsentL1DataGasBounds removes the l1_data_gas bound of a v3 transaction that was
// submitted without one. The spec cannot omit the bound, so such transactions are served
// with a zero one, and only the transaction hash tells them from transactions that set it
// to zero: the hash covers the bound only when it was present.
func dropAbsentL1DataGasBounds(
	transaction core.Transaction,
	resourceBounds map[core.Resource]core.ResourceBounds,
	network *networks.Network,
) error {
	l1DataGas, ok := resourceBounds[core.ResourceL1DataGas]
	if !ok || !l1DataGas.IsZero() {
		return nil
	}

	hash, err := core.TransactionHash(transaction, network)
	if err != nil {
		return fmt.Errorf("hashing transaction: %w", err)
	}

	if !hash.Equal(transaction.Hash()) {
		delete(resourceBounds, core.ResourceL1DataGas)
	}

	return nil
}

func missingFieldError(field string) error {
	return fmt.Errorf("missing %s", field)
}

func unknownVersionError(version *core.TransactionVersion) error {
	return fmt.Errorf("unknown version %s", version)
}

func missingResourceBounds(name string, bounds *rpcv10.ResourceBoundsMap) string {
	if bounds == nil {
		return name
	}
	return cmp.Or(
		missingResourceBound(name+".l1_gas", &bounds.L1Gas),
		missingResourceBound(name+".l2_gas", &bounds.L2Gas),
		missingResourceBound(name+".l1_data_gas", &bounds.L1DataGas),
	)
}

func missingResourceBound(name string, bound *rpcv10.ResourceBounds) string {
	return cmp.Or(
		missing(name+".max_amount", bound.MaxAmount),
		missing(name+".max_price_per_unit", bound.MaxPricePerUnit),
	)
}

func missingOrEmpty(name string, values *felt.Slice[felt.Felt]) string {
	if values == nil || len(*values) == 0 {
		return name
	}
	return ""
}

func feltToUint64(value *felt.Felt) uint64 {
	if value == nil {
		return 0
	}
	return value.Uint64()
}

func derefSlice(slice *felt.Slice[felt.Felt]) felt.Slice[felt.Felt] {
	if slice == nil {
		return nil
	}
	return *slice
}

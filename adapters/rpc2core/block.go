package rpc2core

import (
	"cmp"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils"
)

func AdaptBlock(
	response *rpcv10.BlockWithReceipts,
	network *networks.Network,
) (*core.Block, error) {
	if response == nil {
		return nil, errors.New("nil block")
	}
	if response.Number == nil {
		return nil, errors.New("block header is missing block_number")
	}

	txns := make([]core.Transaction, len(response.Transactions))
	receipts := make([]*core.TransactionReceipt, len(response.Transactions))
	for i := range response.Transactions {
		txnWithReceipt := &response.Transactions[i]
		var err error
		txns[i], err = adaptTransaction(&txnWithReceipt.Transaction, &txnWithReceipt.Receipt, network)
		if err != nil {
			return nil, fmt.Errorf("block %d transaction %d: %w", *response.Number, i, err)
		}
		receipts[i] = adaptReceipt(&txnWithReceipt.Receipt)
	}

	header, err := adaptHeader(&response.BlockHeader, receipts)
	if err != nil {
		return nil, err
	}
	return &core.Block{
		Header:       header,
		Transactions: txns,
		Receipts:     receipts,
	}, nil
}

func adaptTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
	network *networks.Network,
) (core.Transaction, error) {
	if receipt.Hash == nil {
		return nil, errors.New("receipt is missing transaction_hash")
	}
	if field := missingTransactionField(transaction, receipt); field != "" {
		return nil, fmt.Errorf("transaction %s is missing %s", receipt.Hash, field)
	}
	if field := missingReceiptField(receipt); field != "" {
		return nil, fmt.Errorf("receipt of transaction %s is missing %s", receipt.Hash, field)
	}

	var adapted core.Transaction
	var resourceBounds map[core.Resource]core.ResourceBounds
	switch transaction.Type {
	case rpcv10.TxnDeclare:
		declare := adaptDeclareTransaction(transaction, receipt)
		adapted, resourceBounds = declare, declare.ResourceBounds
	case rpcv10.TxnDeploy:
		adapted = adaptDeployTransaction(transaction, receipt)
	case rpcv10.TxnDeployAccount:
		deployAccount := adaptDeployAccountTransaction(transaction, receipt)
		adapted, resourceBounds = deployAccount, deployAccount.ResourceBounds
	case rpcv10.TxnInvoke:
		invoke := adaptInvokeTransaction(transaction, receipt)
		adapted, resourceBounds = invoke, invoke.ResourceBounds
	case rpcv10.TxnL1Handler:
		adapted = adaptL1HandlerTransaction(transaction, receipt)
	default:
		return nil, fmt.Errorf("transaction %s has unknown type %s", receipt.Hash, transaction.Type)
	}
	if err := dropAbsentL1DataGasBounds(adapted, resourceBounds, network); err != nil {
		return nil, err
	}
	return adapted, nil
}

func missingTransactionField(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
) string {
	if transaction.Version == nil {
		return "version"
	}
	signature := missing("signature", transaction.Signature)
	callData := missing("calldata", transaction.CallData)
	constructorCallData := missing("constructor_calldata", transaction.ConstructorCallData)
	contractAddress := missing("contract_address in its receipt", receipt.ContractAddress)

	var field string
	switch transaction.Type {
	case rpcv10.TxnDeclare:
		field = signature
	case rpcv10.TxnDeploy:
		field = cmp.Or(constructorCallData, contractAddress)
	case rpcv10.TxnDeployAccount:
		field = cmp.Or(constructorCallData, signature, contractAddress)
	case rpcv10.TxnInvoke:
		field = cmp.Or(callData, signature)
	case rpcv10.TxnL1Handler:
		// The message hash reads the L1 sender from the first calldata element.
		if transaction.CallData == nil || len(*transaction.CallData) == 0 {
			field = "calldata"
		}
	}
	if field != "" {
		return field
	}
	return missingHashField(transaction)
}

// missingHashField names the first absent field that the transaction or message hash
// dereferences. The hashes of version 0 declares and of deploys are taken as served.
func missingHashField(transaction *rpcv10.Transaction) string {
	version := (*core.TransactionVersion)(transaction.Version)
	contractAddress := missing("contract_address", transaction.ContractAddress)
	senderAddress := missing("sender_address", transaction.SenderAddress)
	entryPointSelector := missing("entry_point_selector", transaction.EntryPointSelector)
	classHash := missing("class_hash", transaction.ClassHash)
	compiledClassHash := missing("compiled_class_hash", transaction.CompiledClassHash)
	salt := missing("contract_address_salt", transaction.ContractAddressSalt)
	maxFee := missing("max_fee", transaction.MaxFee)
	nonce := missing("nonce", transaction.Nonce)
	resourceBounds := missingResourceBound(transaction.ResourceBounds)

	switch transaction.Type {
	case rpcv10.TxnDeclare:
		switch {
		case version.Is(1):
			return cmp.Or(senderAddress, classHash, maxFee, nonce)
		case version.Is(2):
			return cmp.Or(senderAddress, classHash, compiledClassHash, maxFee, nonce)
		case version.Is(3):
			return cmp.Or(senderAddress, classHash, compiledClassHash, nonce, resourceBounds)
		}
	case rpcv10.TxnDeployAccount:
		switch {
		case version.Is(1):
			return cmp.Or(classHash, salt, maxFee, nonce)
		case version.Is(3):
			return cmp.Or(classHash, salt, nonce, resourceBounds)
		}
	case rpcv10.TxnInvoke:
		switch {
		case version.Is(0):
			return cmp.Or(contractAddress, entryPointSelector, maxFee)
		case version.Is(1):
			return cmp.Or(senderAddress, maxFee, nonce)
		case version.Is(3):
			return cmp.Or(senderAddress, nonce, resourceBounds)
		}
	case rpcv10.TxnL1Handler:
		return cmp.Or(contractAddress, entryPointSelector)
	}
	return ""
}

func missingResourceBound(bounds *rpcv10.ResourceBoundsMap) string {
	if bounds == nil {
		return "resource_bounds"
	}
	return cmp.Or(
		missing("resource_bounds.l1_gas.max_amount", bounds.L1Gas.MaxAmount),
		missing("resource_bounds.l1_gas.max_price_per_unit", bounds.L1Gas.MaxPricePerUnit),
		missing("resource_bounds.l2_gas.max_amount", bounds.L2Gas.MaxAmount),
		missing("resource_bounds.l2_gas.max_price_per_unit", bounds.L2Gas.MaxPricePerUnit),
		missing("resource_bounds.l1_data_gas.max_amount", bounds.L1DataGas.MaxAmount),
		missing("resource_bounds.l1_data_gas.max_price_per_unit", bounds.L1DataGas.MaxPricePerUnit),
	)
}

func missingReceiptField(receipt *rpcv10.TransactionReceipt) string {
	if receipt.ActualFee.Amount == nil {
		return "actual_fee.amount"
	}
	for i, event := range receipt.Events {
		if event == nil {
			return fmt.Sprintf("events[%d]", i)
		}
		if event.From == nil {
			return fmt.Sprintf("events[%d].from_address", i)
		}
	}
	for i := range receipt.MessagesSent {
		if receipt.MessagesSent[i].From == nil {
			return fmt.Sprintf("messages_sent[%d].from_address", i)
		}
	}
	return ""
}

func adaptDeclareTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
) *core.DeclareTransaction {
	return &core.DeclareTransaction{
		TransactionHash:       receipt.Hash,
		ClassHash:             transaction.ClassHash,
		SenderAddress:         transaction.SenderAddress,
		MaxFee:                transaction.MaxFee,
		TransactionSignature:  *transaction.Signature,
		Nonce:                 transaction.Nonce,
		Version:               (*core.TransactionVersion)(transaction.Version),
		CompiledClassHash:     transaction.CompiledClassHash,
		ResourceBounds:        adaptResourceBounds(transaction.ResourceBounds),
		Tip:                   feltToUint64(transaction.Tip),
		PaymasterData:         derefSlice(transaction.PaymasterData),
		AccountDeploymentData: derefSlice(transaction.AccountDeploymentData),
		NonceDAMode:           adaptDataAvailabilityMode(transaction.NonceDAMode),
		FeeDAMode:             adaptDataAvailabilityMode(transaction.FeeDAMode),
	}
}

func adaptDeployTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
) *core.DeployTransaction {
	return &core.DeployTransaction{
		TransactionHash:     receipt.Hash,
		ContractAddressSalt: transaction.ContractAddressSalt,
		ContractAddress:     receipt.ContractAddress,
		ClassHash:           transaction.ClassHash,
		ConstructorCallData: *transaction.ConstructorCallData,
		Version:             (*core.TransactionVersion)(transaction.Version),
	}
}

func adaptDeployAccountTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
) *core.DeployAccountTransaction {
	return &core.DeployAccountTransaction{
		DeployTransaction:    *adaptDeployTransaction(transaction, receipt),
		MaxFee:               transaction.MaxFee,
		TransactionSignature: *transaction.Signature,
		Nonce:                transaction.Nonce,
		ResourceBounds:       adaptResourceBounds(transaction.ResourceBounds),
		Tip:                  feltToUint64(transaction.Tip),
		PaymasterData:        derefSlice(transaction.PaymasterData),
		NonceDAMode:          adaptDataAvailabilityMode(transaction.NonceDAMode),
		FeeDAMode:            adaptDataAvailabilityMode(transaction.FeeDAMode),
	}
}

func adaptInvokeTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
) *core.InvokeTransaction {
	return &core.InvokeTransaction{
		TransactionHash:       receipt.Hash,
		CallData:              *transaction.CallData,
		TransactionSignature:  *transaction.Signature,
		MaxFee:                transaction.MaxFee,
		ContractAddress:       transaction.ContractAddress,
		Version:               (*core.TransactionVersion)(transaction.Version),
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
}

func adaptL1HandlerTransaction(
	transaction *rpcv10.Transaction,
	receipt *rpcv10.TransactionReceipt,
) *core.L1HandlerTransaction {
	return &core.L1HandlerTransaction{
		TransactionHash:    receipt.Hash,
		ContractAddress:    transaction.ContractAddress,
		EntryPointSelector: transaction.EntryPointSelector,
		Nonce:              transaction.Nonce,
		CallData:           *transaction.CallData,
		Version:            (*core.TransactionVersion)(transaction.Version),
	}
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
		return fmt.Errorf("hashing transaction %s: %w", transaction.Hash(), err)
	}
	if !hash.Equal(transaction.Hash()) {
		delete(resourceBounds, core.ResourceL1DataGas)
	}
	return nil
}

func adaptReceipt(receipt *rpcv10.TransactionReceipt) *core.TransactionReceipt {
	feeUnit := core.WEI
	if receipt.ActualFee.Unit == rpcv10.FRI {
		feeUnit = core.STRK
	}
	// The RPC receipt only carries the gas totals; steps, builtins, memory holes and the
	// data-availability split are not part of the spec.
	var resources core.ExecutionResources
	resources.TotalGasConsumed = &core.GasConsumed{
		L1Gas:     receipt.ExecutionResources.L1Gas,
		L1DataGas: receipt.ExecutionResources.L1DataGas,
		L2Gas:     receipt.ExecutionResources.L2Gas,
	}
	return &core.TransactionReceipt{
		Fee:                receipt.ActualFee.Amount,
		FeeUnit:            feeUnit,
		Events:             utils.Map(utils.NonNilSlice(receipt.Events), adaptEvent),
		TransactionHash:    receipt.Hash,
		L1ToL2Message:      nil,
		L2ToL1Message:      utils.Map(receipt.MessagesSent, adaptMessageSent),
		Reverted:           receipt.ExecutionStatus == rpcv10.TxnFailure,
		RevertReason:       receipt.RevertReason,
		ExecutionResources: &resources,
	}
}

func adaptEvent(event *rpcv10.Event) *core.Event {
	return (*core.Event)(event)
}

func adaptMessageSent(message rpcv10.MsgToL1) *core.L2ToL1Message {
	to := message.To.Bytes()
	return &core.L2ToL1Message{
		From:    message.From,
		Payload: message.Payload,
		To:      eth.AddressFromBytes(to[:]),
	}
}

func adaptHeader(
	header *rpcv10.BlockHeader,
	receipts []*core.TransactionReceipt,
) (*core.Header, error) {
	if field := cmp.Or(
		missing("block_hash", header.Hash),
		missing("parent_hash", header.ParentHash),
		missing("new_root", header.NewRoot),
		missing("sequencer_address", header.SequencerAddress),
		missing("l1_gas_price.price_in_wei", header.L1GasPrice.InWei),
		missing("l1_gas_price.price_in_fri", header.L1GasPrice.InFri),
		missing("l1_data_gas_price.price_in_wei", header.L1DataGasPrice.InWei),
		missing("l1_data_gas_price.price_in_fri", header.L1DataGasPrice.InFri),
		missing("l2_gas_price.price_in_wei", header.L2GasPrice.InWei),
		missing("l2_gas_price.price_in_fri", header.L2GasPrice.InFri),
	); field != "" {
		return nil, fmt.Errorf("block %d header is missing %s", *header.Number, field)
	}

	var l1DAMode core.L1DAMode
	switch header.L1DAMode {
	case rpcv10.Blob:
		l1DAMode = core.Blob
	case rpcv10.Calldata:
		l1DAMode = core.Calldata
	default:
		return nil, fmt.Errorf(
			"block %d header has unknown l1_da_mode %d", *header.Number, header.L1DAMode,
		)
	}

	eventCount := uint64(0)
	for _, receipt := range receipts {
		eventCount += uint64(len(receipt.Events))
	}
	return &core.Header{
		Hash:             header.Hash,
		ParentHash:       header.ParentHash,
		Number:           *header.Number,
		GlobalStateRoot:  header.NewRoot,
		SequencerAddress: header.SequencerAddress,
		TransactionCount: uint64(len(receipts)),
		EventCount:       eventCount,
		Timestamp:        header.Timestamp,
		ProtocolVersion:  header.StarknetVersion,
		EventsBloom:      core.EventsBloom(receipts),
		L1GasPriceETH:    header.L1GasPrice.InWei,
		L1GasPriceSTRK:   header.L1GasPrice.InFri,
		L1DAMode:         l1DAMode,
		L1DataGasPrice:   adaptGasPrice(&header.L1DataGasPrice),
		L2GasPrice:       adaptGasPrice(&header.L2GasPrice),
		Signatures:       [][]*felt.Felt{},
	}, nil
}

func adaptGasPrice(price *rpcv10.ResourcePrice) *core.GasPrice {
	return &core.GasPrice{
		PriceInWei: price.InWei,
		PriceInFri: price.InFri,
	}
}

// missing returns the field's name when its value is absent, so cmp.Or can pick the first
// absent one.
func missing[T any](name string, value *T) string {
	if value == nil {
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

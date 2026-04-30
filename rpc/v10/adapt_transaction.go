package rpcv10

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/gateway"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
)

var ErrTransactionNotFound = errors.New("transaction not found")

// AddTxGatewayPayload embeds a starknet.Transaction along with an optional ContractClass.
type AddTxGatewayPayload struct {
	starknet.Transaction
	ContractClass json.RawMessage `json:"contract_class,omitempty"`
	Proof         core.Base64     `json:"proof,omitempty"`
}

// AdaptCoreTransaction adapts a core.Transaction to a local *Transaction.
func AdaptCoreTransaction(t core.Transaction) *Transaction {
	var txn *Transaction
	switch v := t.(type) {
	case *core.DeployTransaction:
		txn = &Transaction{
			Type:                TxnDeploy,
			Hash:                v.Hash(),
			ClassHash:           v.ClassHash,
			Version:             v.Version.AsFelt(),
			ContractAddressSalt: v.ContractAddressSalt,
			ConstructorCallData: &v.ConstructorCallData,
		}
	case *core.InvokeTransaction:
		txn = adaptInvokeTransaction(v)
	case *core.DeclareTransaction:
		txn = adaptDeclareTransaction(v)
	case *core.DeployAccountTransaction:
		txn = adaptDeployAccountTransaction(v)
	case *core.L1HandlerTransaction:
		nonce := v.Nonce
		if nonce == nil {
			nonce = &felt.Zero
		}
		txn = &Transaction{
			Type:               TxnL1Handler,
			Hash:               v.Hash(),
			Version:            v.Version.AsFelt(),
			Nonce:              nonce,
			ContractAddress:    v.ContractAddress,
			EntryPointSelector: v.EntryPointSelector,
			CallData:           &v.CallData,
		}
	default:
		panic("not a transaction")
	}

	if txn.Version.IsZero() && txn.Type != TxnL1Handler {
		txn.Nonce = nil
	}
	return txn
}

func adaptCoreResourceBounds(rb map[core.Resource]core.ResourceBounds) ResourceBoundsMap {
	// Check if L1DataGas exists in the map
	var l1DataGasResourceBounds ResourceBounds
	if _, ok := rb[core.ResourceL1DataGas]; ok {
		l1DataGasResourceBounds = ResourceBounds{
			MaxAmount:       felt.NewFromUint64[felt.Felt](rb[core.ResourceL1DataGas].MaxAmount),
			MaxPricePerUnit: rb[core.ResourceL1DataGas].MaxPricePerUnit,
		}
	} else {
		l1DataGasResourceBounds = ResourceBounds{
			MaxAmount:       &felt.Zero,
			MaxPricePerUnit: &felt.Zero,
		}
	}

	// As L1Gas & L2Gas will always be present, we can directly assign them
	rpcResourceBounds := ResourceBoundsMap{
		L1Gas: ResourceBounds{
			MaxAmount:       felt.NewFromUint64[felt.Felt](rb[core.ResourceL1Gas].MaxAmount),
			MaxPricePerUnit: rb[core.ResourceL1Gas].MaxPricePerUnit,
		},
		L2Gas: ResourceBounds{
			MaxAmount:       felt.NewFromUint64[felt.Felt](rb[core.ResourceL2Gas].MaxAmount),
			MaxPricePerUnit: rb[core.ResourceL2Gas].MaxPricePerUnit,
		},
		L1DataGas: l1DataGasResourceBounds,
	}
	return rpcResourceBounds
}

// AdaptRPCTxToFeederTx adapts a Transaction to a starknet.Transaction.
// No nil checks here since this function is only called with broadcasted txs, and
// the fields are required by the 'validate' tag.
func AdaptRPCTxToFeederTx(rpcTx *Transaction) starknet.Transaction {
	resourceBounds := make(map[starknet.Resource]starknet.ResourceBounds)
	resourceBounds[starknet.ResourceL1Gas] = starknet.ResourceBounds{
		MaxAmount:       rpcTx.ResourceBounds.L1Gas.MaxAmount,
		MaxPricePerUnit: rpcTx.ResourceBounds.L1Gas.MaxPricePerUnit,
	}
	resourceBounds[starknet.ResourceL2Gas] = starknet.ResourceBounds{
		MaxAmount:       rpcTx.ResourceBounds.L2Gas.MaxAmount,
		MaxPricePerUnit: rpcTx.ResourceBounds.L2Gas.MaxPricePerUnit,
	}
	resourceBounds[starknet.ResourceL1DataGas] = starknet.ResourceBounds{
		MaxAmount:       rpcTx.ResourceBounds.L1DataGas.MaxAmount,
		MaxPricePerUnit: rpcTx.ResourceBounds.L1DataGas.MaxPricePerUnit,
	}

	return starknet.Transaction{
		Hash:                  rpcTx.Hash,
		Version:               rpcTx.Version,
		ContractAddress:       rpcTx.ContractAddress,
		ContractAddressSalt:   rpcTx.ContractAddressSalt,
		ClassHash:             rpcTx.ClassHash,
		ConstructorCallData:   rpcTx.ConstructorCallData,
		Type:                  starknet.TransactionType(rpcTx.Type),
		SenderAddress:         rpcTx.SenderAddress,
		MaxFee:                rpcTx.MaxFee,
		Signature:             rpcTx.Signature,
		CallData:              rpcTx.CallData,
		EntryPointSelector:    rpcTx.EntryPointSelector,
		Nonce:                 rpcTx.Nonce,
		CompiledClassHash:     rpcTx.CompiledClassHash,
		ResourceBounds:        &resourceBounds,
		Tip:                   rpcTx.Tip,
		NonceDAMode:           (*starknet.DataAvailabilityMode)(rpcTx.NonceDAMode),
		FeeDAMode:             (*starknet.DataAvailabilityMode)(rpcTx.FeeDAMode),
		AccountDeploymentData: rpcTx.AccountDeploymentData,
		PaymasterData:         rpcTx.PaymasterData,
		ProofFacts:            rpcTx.ProofFacts,
	}
}

// AdaptReceipt adapts a receipt and transaction into a local *TransactionReceipt.
// todo(rdr): TransactionReceipt should be returned by value
func AdaptReceipt(
	receipt *core.TransactionReceipt,
	txn core.Transaction,
	finalityStatus TxnFinalityStatus,
) *TransactionReceipt {
	messages := make([]*MsgToL1, len(receipt.L2ToL1Message))
	for idx, msg := range receipt.L2ToL1Message {
		messages[idx] = &MsgToL1{
			To:      msg.To,
			Payload: msg.Payload,
			From:    msg.From,
		}
	}

	events := make([]*Event, len(receipt.Events))
	for idx, event := range receipt.Events {
		events[idx] = &Event{
			From: event.From,
			Keys: event.Keys,
			Data: event.Data,
		}
	}

	var messageHash string
	var contractAddress *felt.Felt
	switch v := txn.(type) {
	case *core.DeployTransaction:
		contractAddress = v.ContractAddress
	case *core.DeployAccountTransaction:
		contractAddress = v.ContractAddress
	case *core.L1HandlerTransaction:
		messageHash = "0x" + hex.EncodeToString(v.MessageHash())
	}

	var es TxnExecutionStatus
	if receipt.Reverted {
		es = TxnFailure
	} else {
		es = TxnSuccess
	}

	return &TransactionReceipt{
		FinalityStatus:  finalityStatus,
		ExecutionStatus: es,
		Type:            AdaptCoreTransaction(txn).Type,
		Hash:            txn.Hash(),
		ActualFee: &FeePayment{
			Amount: receipt.Fee,
			Unit:   feeUnit(txn),
		},
		MessagesSent:       messages,
		Events:             events,
		ContractAddress:    contractAddress,
		RevertReason:       receipt.RevertReason,
		ExecutionResources: adaptExecutionResources(receipt.ExecutionResources),
		MessageHash:        messageHash,
	}
}

// AdaptReceiptWithBlockInfo returns JSON-RPC TXN_RECEIPT_WITH_BLOCK_INFO.
func AdaptReceiptWithBlockInfo(
	receipt *core.TransactionReceipt,
	txn core.Transaction,
	finalityStatus TxnFinalityStatus,
	blockHash *felt.Felt,
	blockNumber uint64,
) *TransactionReceipt {
	adaptedReceipt := AdaptReceipt(receipt, txn, finalityStatus)

	// Assign block number for canonical, pre_latest and pre_confirmed block
	adaptedReceipt.BlockNumber = &blockNumber
	adaptedReceipt.BlockHash = blockHash

	return adaptedReceipt
}

func AdaptTransactionStatus(
	txStatus *starknet.TransactionStatus,
) (TransactionStatus, error) {
	var status TransactionStatus

	switch finalityStatus := txStatus.FinalityStatus; finalityStatus {
	case starknet.AcceptedOnL1:
		status.Finality = TxnStatusAcceptedOnL1
	case starknet.AcceptedOnL2:
		status.Finality = TxnStatusAcceptedOnL2
	case starknet.Received:
		status.Finality = TxnStatusReceived
	case starknet.PreConfirmed:
		status.Finality = TxnStatusPreConfirmed
	case starknet.Candidate:
		status.Finality = TxnStatusCandidate
		// Candidate transaction does not have execution_status yet
		return status, nil
	case starknet.NotReceived:
		return TransactionStatus{}, ErrTransactionNotFound
	default:
		return TransactionStatus{}, fmt.Errorf("unknown finality status: %v", finalityStatus)
	}

	switch txStatus.ExecutionStatus {
	case starknet.Succeeded:
		status.Execution = TxnSuccess
	case starknet.Reverted:
		status.Execution = TxnFailure
		status.FailureReason = txStatus.TxRevertReason
	case starknet.Rejected:
		// Upon querying historical transaction, gateway returns `RECEIVED` finality status,
		// along with `REJECTED` execution status. Rejected status is not supported by spec 0.9.0,
		// `REJECTED` status is mapped to `errTransactionNotFound`.
		return TransactionStatus{}, ErrTransactionNotFound
	default: // Omit the field on error. It's optional in the spec.
	}

	return status, nil
}

func adaptInvokeTransaction(t *core.InvokeTransaction) *Transaction {
	tx := &Transaction{
		Type:               TxnInvoke,
		Hash:               t.Hash(),
		MaxFee:             t.MaxFee,
		Version:            t.Version.AsFelt(),
		Signature:          new(t.Signature()),
		Nonce:              t.Nonce,
		CallData:           &t.CallData,
		ContractAddress:    t.ContractAddress,
		SenderAddress:      t.SenderAddress,
		EntryPointSelector: t.EntryPointSelector,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = new(adaptCoreResourceBounds(t.ResourceBounds))
		tx.Tip = felt.NewFromUint64[felt.Felt](t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.AccountDeploymentData = &t.AccountDeploymentData
		tx.NonceDAMode = new(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = new(DataAvailabilityMode(t.FeeDAMode))

		if t.ProofFacts != nil {
			tx.ProofFacts = &t.ProofFacts
		}
	}
	return tx
}

func adaptDeclareTransaction(t *core.DeclareTransaction) *Transaction {
	tx := &Transaction{
		Hash:              t.Hash(),
		Type:              TxnDeclare,
		MaxFee:            t.MaxFee,
		Version:           t.Version.AsFelt(),
		Signature:         new(t.Signature()),
		Nonce:             t.Nonce,
		ClassHash:         t.ClassHash,
		SenderAddress:     t.SenderAddress,
		CompiledClassHash: t.CompiledClassHash,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = new(adaptCoreResourceBounds(t.ResourceBounds))
		tx.Tip = felt.NewFromUint64[felt.Felt](t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.AccountDeploymentData = &t.AccountDeploymentData
		tx.NonceDAMode = new(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = new(DataAvailabilityMode(t.FeeDAMode))
	}

	return tx
}

func adaptDeployAccountTransaction(t *core.DeployAccountTransaction) *Transaction {
	tx := &Transaction{
		Hash:                t.Hash(),
		MaxFee:              t.MaxFee,
		Version:             t.Version.AsFelt(),
		Signature:           new(t.Signature()),
		Nonce:               t.Nonce,
		Type:                TxnDeployAccount,
		ContractAddressSalt: t.ContractAddressSalt,
		ConstructorCallData: &t.ConstructorCallData,
		ClassHash:           t.ClassHash,
	}

	if tx.Version.Uint64() == 3 {
		tx.ResourceBounds = new(adaptCoreResourceBounds(t.ResourceBounds))
		tx.Tip = felt.NewFromUint64[felt.Felt](t.Tip)
		tx.PaymasterData = &t.PaymasterData
		tx.NonceDAMode = new(DataAvailabilityMode(t.NonceDAMode))
		tx.FeeDAMode = new(DataAvailabilityMode(t.FeeDAMode))
	}

	return tx
}

func adaptContractClassToStarknet(class *ContractClass) starknet.SierraClass {
	handleEntryPoints := func(
		entryPoints []ContractClassEntryPoint,
	) []starknet.SierraEntryPoint {
		starknetEntryPoints := make([]starknet.SierraEntryPoint, len(entryPoints))
		for i, entryPoint := range entryPoints {
			starknetEntryPoints[i] = starknet.SierraEntryPoint{
				Index:    *entryPoint.Index,
				Selector: entryPoint.Selector,
			}
		}
		return starknetEntryPoints
	}

	return starknet.SierraClass{
		Abi:     class.ABI,
		Version: class.ContractClassVersion,
		Program: utils.ToPtrSlice(class.SierraProgram),
		EntryPoints: starknet.SierraEntryPoints{
			Constructor: handleEntryPoints(class.EntryPoints.Constructor),
			External:    handleEntryPoints(class.EntryPoints.External),
			L1Handler:   handleEntryPoints(class.EntryPoints.L1Handler),
		},
	}
}

//nolint:gocyclo // maps gateway error codes to RPC errors
func MakeJSONErrorFromGatewayError(err error) *jsonrpc.Error {
	gatewayErr, ok := err.(*gateway.Error)
	if !ok {
		return jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}

	switch gatewayErr.Code {
	case gateway.InvalidContractClass:
		return rpccore.ErrInvalidContractClass
	case gateway.UndeclaredClass:
		return rpccore.ErrClassHashNotFound
	case gateway.ClassAlreadyDeclared:
		return rpccore.ErrClassAlreadyDeclared
	case gateway.InsufficientResourcesForValidate:
		return rpccore.ErrInsufficientResourcesForValidate
	case gateway.InsufficientAccountBalance:
		return rpccore.ErrInsufficientAccountBalanceV0_8
	case gateway.ValidateFailure:
		if strings.Contains(gatewayErr.Message, rpccore.ErrInvalidTransactionNonce.Message) {
			return rpccore.ErrInvalidTransactionNonce.CloneWithData(gatewayErr.Message)
		} else {
			return rpccore.ErrValidationFailure.CloneWithData(gatewayErr.Message)
		}
	case gateway.ContractBytecodeSizeTooLarge, gateway.ContractClassObjectSizeTooLarge:
		return rpccore.ErrContractClassSizeTooLarge
	case gateway.DuplicatedTransaction:
		return rpccore.ErrDuplicateTx
	case gateway.InvalidTransactionNonce:
		return rpccore.ErrInvalidTransactionNonce.CloneWithData(gatewayErr.Message)
	case gateway.CompilationFailed:
		return rpccore.ErrCompilationFailed.CloneWithData(gatewayErr.Message)
	case gateway.InvalidCompiledClassHash:
		return rpccore.ErrCompiledClassHashMismatch
	case gateway.InvalidTransactionVersion:
		return rpccore.ErrUnsupportedTxVersion
	case gateway.InvalidContractClassVersion:
		return rpccore.ErrUnsupportedContractClassVersion
	case gateway.ReplacementTransactionUnderPriced:
		return rpccore.ErrReplacementTransactionUnderPriced
	case gateway.FeeBelowMinimum:
		return rpccore.ErrFeeBelowMinimum
	case gateway.InvalidProof:
		return rpccore.ErrInvalidProof
	default:
		return rpccore.ErrUnexpectedError.CloneWithData(gatewayErr.Message)
	}
}

func adaptEntryPointsToCore(eps []ContractClassEntryPoint) []core.SierraEntryPoint {
	result := make([]core.SierraEntryPoint, len(eps))
	for i, ep := range eps {
		result[i] = core.SierraEntryPoint{
			Selector: ep.Selector,
			Index:    *ep.Index,
		}
	}
	return result
}

func adaptResourceBoundsToCore(
	rb *ResourceBoundsMap,
) map[core.Resource]core.ResourceBounds {
	coreResourceBounds := make(map[core.Resource]core.ResourceBounds)
	coreResourceBounds[core.ResourceL1Gas] = core.ResourceBounds{
		MaxAmount:       rb.L1Gas.MaxAmount.Uint64(),
		MaxPricePerUnit: rb.L1Gas.MaxPricePerUnit,
	}
	coreResourceBounds[core.ResourceL2Gas] = core.ResourceBounds{
		MaxAmount:       rb.L2Gas.MaxAmount.Uint64(),
		MaxPricePerUnit: rb.L2Gas.MaxPricePerUnit,
	}
	coreResourceBounds[core.ResourceL1DataGas] = core.ResourceBounds{
		MaxAmount:       rb.L1DataGas.MaxAmount.Uint64(),
		MaxPricePerUnit: rb.L1DataGas.MaxPricePerUnit,
	}

	return coreResourceBounds
}

func adaptBroadcastedInvokeToCore(tx *BroadcastedTransaction) *core.InvokeTransaction {
	return &core.InvokeTransaction{
		TransactionHash:       nil, // it will be set later
		CallData:              *tx.CallData,
		TransactionSignature:  *tx.Signature,
		Nonce:                 tx.Nonce,
		Version:               (*core.TransactionVersion)(tx.Version),
		SenderAddress:         tx.SenderAddress,
		ResourceBounds:        adaptResourceBoundsToCore(tx.ResourceBounds),
		Tip:                   tx.Tip.Uint64(),
		PaymasterData:         *tx.PaymasterData,
		AccountDeploymentData: *tx.AccountDeploymentData,
		NonceDAMode:           core.DataAvailabilityMode(*tx.NonceDAMode),
		FeeDAMode:             core.DataAvailabilityMode(*tx.FeeDAMode),
		ProofFacts:            utils.DerefSlice(tx.ProofFacts),
		MaxFee:                nil, // not present in v3 invoke
		ContractAddress:       nil, // not present in v3 invoke
		EntryPointSelector:    nil, // not present in v3 invoke
	}
}

func adaptBroadcastedDeclareToCore(
	tx *BroadcastedTransaction,
	classHash *felt.Felt,
) *core.DeclareTransaction {
	return &core.DeclareTransaction{
		TransactionHash:       nil, // it will be set later
		ClassHash:             classHash,
		SenderAddress:         tx.SenderAddress,
		TransactionSignature:  *tx.Signature,
		Nonce:                 tx.Nonce,
		Version:               (*core.TransactionVersion)(tx.Version),
		CompiledClassHash:     tx.CompiledClassHash,
		ResourceBounds:        adaptResourceBoundsToCore(tx.ResourceBounds),
		Tip:                   tx.Tip.Uint64(),
		PaymasterData:         *tx.PaymasterData,
		AccountDeploymentData: *tx.AccountDeploymentData,
		NonceDAMode:           core.DataAvailabilityMode(*tx.NonceDAMode),
		FeeDAMode:             core.DataAvailabilityMode(*tx.FeeDAMode),
		MaxFee:                nil, // not present in v3 declare
	}
}

func adaptBroadcastedDeployAccountToCore(
	tx *BroadcastedTransaction,
) *core.DeployAccountTransaction {
	return &core.DeployAccountTransaction{
		DeployTransaction: core.DeployTransaction{
			TransactionHash:     nil, // it will be set later
			ContractAddressSalt: tx.ContractAddressSalt,
			ClassHash:           tx.ClassHash,
			ConstructorCallData: *tx.ConstructorCallData,
			Version:             (*core.TransactionVersion)(tx.Version),
			ContractAddress:     nil, // not present in v3 deploy account
		},
		MaxFee:               nil, // not present in v3 deploy account
		TransactionSignature: *tx.Signature,
		Nonce:                tx.Nonce,
		ResourceBounds:       adaptResourceBoundsToCore(tx.ResourceBounds),
		Tip:                  tx.Tip.Uint64(),
		PaymasterData:        *tx.PaymasterData,
		NonceDAMode:          core.DataAvailabilityMode(*tx.NonceDAMode),
		FeeDAMode:            core.DataAvailabilityMode(*tx.FeeDAMode),
	}
}

// AdaptBroadcastedTransactionToCore adapts a BroadcastedTransaction to a core.Transaction.
// It also populates the transaction hash, and the class hash for Declare transactions.
// @todo add tests for this function.
func AdaptBroadcastedTransactionToCore(
	ctx context.Context,
	broadcastedTxn *BroadcastedTransaction,
	network *networks.Network,
) (core.Transaction, error) {
	var coreTx core.Transaction

	txType := broadcastedTxn.Type
	switch txType {
	case TxnInvoke:
		txn := adaptBroadcastedInvokeToCore(broadcastedTxn)
		txnHash, err := core.TransactionHash(txn, network)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate transaction hash: %w", err)
		}
		txn.TransactionHash = &txnHash
		coreTx = txn
	case TxnDeclare:
		classHash, err := core.CalculateSierraClassHash(
			utils.ToPtrSlice(broadcastedTxn.ContractClass.SierraProgram),
			broadcastedTxn.ContractClass.ContractClassVersion,
			adaptEntryPointsToCore(broadcastedTxn.ContractClass.EntryPoints.Constructor),
			adaptEntryPointsToCore(broadcastedTxn.ContractClass.EntryPoints.External),
			adaptEntryPointsToCore(broadcastedTxn.ContractClass.EntryPoints.L1Handler),
			broadcastedTxn.ContractClass.ABI,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate transaction hash: %w", err)
		}
		txn := adaptBroadcastedDeclareToCore(broadcastedTxn, &classHash)

		txnHash, err := core.TransactionHash(txn, network)
		if err != nil {
			return nil, err
		}
		txn.TransactionHash = &txnHash
		coreTx = txn
	case TxnDeployAccount:
		txn := adaptBroadcastedDeployAccountToCore(broadcastedTxn)
		txnHash, err := core.TransactionHash(txn, network)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate transaction hash: %w", err)
		}
		txn.TransactionHash = &txnHash
		coreTx = txn
	default:
		return nil, fmt.Errorf("invalid transaction type %q", txType)
	}

	return coreTx, nil
}

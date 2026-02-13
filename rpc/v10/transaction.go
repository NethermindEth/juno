package rpcv10

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

type Transaction struct {
	rpcv9.Transaction
	ProofFacts []*felt.Felt `json:"proof_facts,omitempty"`
}

func AdaptTransaction(coreTx core.Transaction, includeProofFacts bool) Transaction {
	v9Tx := rpcv9.AdaptTransaction(coreTx)
	tx := Transaction{
		Transaction: *v9Tx,
	}

	if includeProofFacts {
		if invokeTx, ok := coreTx.(*core.InvokeTransaction); ok {
			if invokeTx.Version.Is(3) && invokeTx.ProofFacts != nil {
				tx.ProofFacts = invokeTx.ProofFacts
			}
		}
	}

	return tx
}

type BroadcastedTransaction struct {
	rpcv9.BroadcastedTransaction
	Proof      []uint64    `json:"proof,omitempty"`
	ProofFacts []felt.Felt `json:"proof_facts,omitempty"`
}

func AdaptBroadcastedTransaction(
	broadcastedTxn *BroadcastedTransaction,
	network *utils.Network,
) (core.Transaction, core.ClassDefinition, *felt.Felt, error) {
	isInvokeV3 := broadcastedTxn.Transaction.Type == rpcv9.TxnInvoke &&
		isVersion3(broadcastedTxn.Transaction.Version)

	if !isInvokeV3 {
		if len(broadcastedTxn.ProofFacts) > 0 {
			return nil, nil, nil, fmt.Errorf("proof_facts can only be included in invoke v3 transactions")
		}
		if len(broadcastedTxn.Proof) > 0 {
			return nil, nil, nil, fmt.Errorf("proof can only be included in invoke v3 transactions")
		}
	}

	feederTxn := rpcv9.AdaptRPCTxToFeederTx(&broadcastedTxn.Transaction)
	if isInvokeV3 && len(broadcastedTxn.ProofFacts) > 0 {
		proofFactsPtrs := make([]*felt.Felt, len(broadcastedTxn.ProofFacts))
		for i := range broadcastedTxn.ProofFacts {
			copy := broadcastedTxn.ProofFacts[i]
			proofFactsPtrs[i] = &copy
		}
		feederTxn.ProofFacts = &proofFactsPtrs
	}

	txn, err := sn2core.AdaptTransaction(&feederTxn)
	if err != nil {
		return nil, nil, nil, err
	}

	var declaredClass core.ClassDefinition
	if len(broadcastedTxn.ContractClass) != 0 {
		declaredClass, err = rpcv9.AdaptDeclaredClass(broadcastedTxn.ContractClass)
		if err != nil {
			return nil, nil, nil, err
		}
	} else if broadcastedTxn.Type == rpcv9.TxnDeclare {
		return nil, nil, nil, errors.New("declare without a class definition")
	}

	if t, ok := txn.(*core.DeclareTransaction); ok {
		classHash, err := declaredClass.Hash()
		if err != nil {
			return nil, nil, nil, err
		}
		t.ClassHash = &classHash
	}

	txnHash, err := core.TransactionHash(txn, network)
	if err != nil {
		return nil, nil, nil, err
	}

	var paidFeeOnL1 *felt.Felt
	switch t := txn.(type) {
	case *core.DeclareTransaction:
		t.TransactionHash = &txnHash
	case *core.InvokeTransaction:
		t.TransactionHash = &txnHash
	case *core.DeployAccountTransaction:
		t.TransactionHash = &txnHash
	case *core.L1HandlerTransaction:
		t.TransactionHash = &txnHash
		paidFeeOnL1 = broadcastedTxn.PaidFeeOnL1
	default:
		return nil, nil, nil, errors.New("unsupported transaction")
	}

	if txn.Hash() == nil {
		return nil, nil, nil, errors.New("deprecated transaction type")
	}

	return txn, declaredClass, paidFeeOnL1, nil
}

type AddTxResponse struct {
	TransactionHash felt.TransactionHash `json:"transaction_hash"`
	ContractAddress *felt.Address        `json:"contract_address,omitempty"`
	ClassHash       *felt.ClassHash      `json:"class_hash,omitempty"`
}

// AddTransaction adds a transaction to the mempool or forwards it to the feeder gateway.
func (h *Handler) AddTransaction(
	ctx context.Context,
	tx *BroadcastedTransaction,
) (AddTxResponse, *jsonrpc.Error) {
	var (
		res AddTxResponse
		err *jsonrpc.Error
	)
	if h.memPool != nil {
		res, err = h.addToMempool(ctx, tx)
	} else {
		res, err = h.pushToFeederGateway(ctx, tx)
	}

	if err != nil {
		return AddTxResponse{}, err
	}

	if h.submittedTransactionsCache != nil {
		h.submittedTransactionsCache.Add((*felt.Felt)(&res.TransactionHash))
	}

	return res, nil
}

func (h *Handler) addToMempool(
	ctx context.Context,
	tx *BroadcastedTransaction,
) (AddTxResponse, *jsonrpc.Error) {
	userTxn, userClass, paidFeeOnL1, err := AdaptBroadcastedTransaction(tx, h.bcReader.Network())
	if err != nil {
		return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(err.Error())
	}
	if err = h.memPool.Push(ctx, &mempool.BroadcastedTransaction{
		Transaction:   userTxn,
		DeclaredClass: userClass,
		PaidFeeOnL1:   paidFeeOnL1,
	}); err != nil {
		return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(err.Error())
	}
	userTxnHash := felt.TransactionHash(*userTxn.Hash())
	res := AddTxResponse{TransactionHash: userTxnHash}
	switch tx.Type {
	case rpcv9.TxnDeployAccount:
		contractAddress := core.ContractAddress(
			&felt.Zero,
			tx.ClassHash,
			tx.ContractAddressSalt,
			*tx.ConstructorCallData,
		)
		res.ContractAddress = (*felt.Address)(&contractAddress)
	case rpcv9.TxnDeclare:
		classHash, err := userClass.Hash()
		if err != nil {
			return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(err.Error())
		}
		res.ClassHash = (*felt.ClassHash)(&classHash)
	}
	return res, nil
}

func (h *Handler) pushToFeederGateway(
	ctx context.Context,
	tx *BroadcastedTransaction,
) (AddTxResponse, *jsonrpc.Error) {
	v9Tx := &tx.BroadcastedTransaction
	if v9Tx.Transaction.Type == rpcv9.TxnDeclare &&
		v9Tx.Transaction.Version.Cmp(felt.NewFromUint64[felt.Felt](2)) != -1 {
		contractClass := make(map[string]any)
		if err := json.Unmarshal(v9Tx.ContractClass, &contractClass); err != nil {
			return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(
				fmt.Sprintf("unmarshal contract class: %v", err),
			)
		}
		sierraProg, ok := contractClass["sierra_program"]
		if !ok {
			return AddTxResponse{}, jsonrpc.Err(
				jsonrpc.InvalidParams,
				"{'sierra_program': ['Missing data for required field.']}",
			)
		}

		sierraProgBytes, errIn := json.Marshal(sierraProg)
		if errIn != nil {
			return AddTxResponse{}, jsonrpc.Err(jsonrpc.InternalError, errIn.Error())
		}

		gwSierraProg, errIn := utils.Gzip64Encode(sierraProgBytes)
		if errIn != nil {
			return AddTxResponse{}, jsonrpc.Err(jsonrpc.InternalError, errIn.Error())
		}

		contractClass["sierra_program"] = gwSierraProg
		newContractClass, err := json.Marshal(contractClass)
		if err != nil {
			return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(
				fmt.Sprintf("marshal revised contract class: %v", err),
			)
		}
		v9Tx.ContractClass = newContractClass
	}

	payload := adaptRPCTxToFeederPayload(tx)
	txJSON, err := json.Marshal(&payload)
	if err != nil {
		return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(
			fmt.Sprintf("marshal transaction: %v", err),
		)
	}

	if h.gatewayClient == nil {
		return AddTxResponse{}, rpccore.ErrInternal.CloneWithData("no gateway client configured")
	}

	respJSON, err := h.gatewayClient.AddTransaction(ctx, txJSON)
	if err != nil {
		jsonErr := rpcv9.MakeJSONErrorFromGatewayError(err)
		return AddTxResponse{}, jsonErr
	}

	var gatewayResponse struct {
		TransactionHash felt.TransactionHash `json:"transaction_hash"`
		ContractAddress *felt.Address        `json:"address"`
		ClassHash       *felt.ClassHash      `json:"class_hash"`
	}
	if err = json.Unmarshal(respJSON, &gatewayResponse); err != nil {
		return AddTxResponse{}, jsonrpc.Err(
			jsonrpc.InternalError,
			fmt.Sprintf("unmarshal gateway response: %v", err),
		)
	}

	return AddTxResponse{
		TransactionHash: gatewayResponse.TransactionHash,
		ContractAddress: gatewayResponse.ContractAddress,
		ClassHash:       gatewayResponse.ClassHash,
	}, nil
}

type addTxGatewayPayload struct {
	rpcv9.AddTxGatewayPayload
	Proof      []uint64    `json:"proof,omitempty"`
	ProofFacts []felt.Felt `json:"proof_facts,omitempty"`
}

func adaptRPCTxToFeederPayload(
	tx *BroadcastedTransaction,
) addTxGatewayPayload {
	var payload addTxGatewayPayload
	rpcv9Payload := rpcv9.AdaptRPCTxToAddTxGatewayPayload(&tx.BroadcastedTransaction)
	payload.AddTxGatewayPayload = rpcv9Payload
	if tx.Transaction.Type == rpcv9.TxnInvoke && isVersion3(tx.Transaction.Version) {
		payload.Proof = tx.Proof
		payload.ProofFacts = tx.ProofFacts
	}
	return payload
}

func (h *Handler) TransactionStatus(
	ctx context.Context,
	hash *felt.Felt,
) (rpcv9.TransactionStatus, *jsonrpc.Error) {
	receipt, txErr := h.TransactionReceiptByHash(hash)
	switch txErr {
	case nil:
		return rpcv9.TransactionStatus{
			Finality:      rpcv9.TxnStatus(receipt.FinalityStatus),
			Execution:     receipt.ExecutionStatus,
			FailureReason: receipt.RevertReason,
		}, nil
	case rpccore.ErrTxnHashNotFound:
		// Search pre-confirmed block for 'CANDIDATE' status
		var txStatus *starknet.TransactionStatus
		var err error
		preConfirmedB, err := h.PendingData()

		if err == nil {
			for _, txn := range preConfirmedB.GetCandidateTransaction() {
				if txn.Hash().Equal(hash) {
					txStatus = &starknet.TransactionStatus{FinalityStatus: starknet.Candidate}
					break
				}
			}
		}
		// Not Candidate
		if txStatus == nil {
			if h.feederClient == nil {
				break
			}

			txStatus, err = h.feederClient.Transaction(ctx, hash)
			if err != nil {
				return rpcv9.TransactionStatus{}, jsonrpc.Err(jsonrpc.InternalError, err.Error())
			}

			if txStatus.FinalityStatus == starknet.NotReceived && h.submittedTransactionsCache != nil {
				if h.submittedTransactionsCache.Contains(hash) {
					txStatus.FinalityStatus = starknet.Received
				}
			}
		}

		status, err := rpcv9.AdaptTransactionStatus(txStatus)
		if err != nil {
			if !errors.Is(err, rpcv9.ErrTransactionNotFound) {
				h.log.Error("Failed to adapt transaction status", zap.Error(err))
			}
			return rpcv9.TransactionStatus{}, rpccore.ErrTxnHashNotFound
		}
		return status, nil
	}
	return rpcv9.TransactionStatus{}, txErr
}

/****************************************************
		Transaction Handlers
*****************************************************/

// TransactionByHash returns the details of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/0bf403bfafbfbe0eaa52103a9c7df545bec8f73b/api/starknet_api_openrpc.json#L315 //nolint:lll
//
//nolint:lll // URL exceeds line
func (h *Handler) TransactionByHash(
	hash *felt.Felt,
	responseFlags ResponseFlags,
) (*Transaction, *jsonrpc.Error) {
	// Check pending data
	if pending, err := h.PendingData(); err == nil {
		if txn, err := pending.TransactionByHash(hash); err == nil {
			adaptedTxn := AdaptTransaction(txn, responseFlags.IncludeProofFacts)
			return &adaptedTxn, nil
		}
	}

	txn, err := h.bcReader.TransactionByHash(hash)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return nil, rpccore.ErrTxnHashNotFound
	}
	adaptedTxn := AdaptTransaction(txn, responseFlags.IncludeProofFacts)
	return &adaptedTxn, nil
}

// TransactionByBlockIDAndIndex returns the details of a transaction identified by the given
// BlockID and index.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/0bf403bfafbfbe0eaa52103a9c7df545bec8f73b/api/starknet_api_openrpc.json#L342 //nolint:lll
//
//nolint:lll // URL exceeds line limit
func (h *Handler) TransactionByBlockIDAndIndex(
	blockID *rpcv9.BlockID, txIndex int, responseFlags ResponseFlags,
) (*Transaction, *jsonrpc.Error) {
	includeProofFacts := responseFlags.IncludeProofFacts

	if txIndex < 0 {
		return nil, rpccore.ErrInvalidTxIndex
	}

	var blockNumber uint64
	var err error
	switch {
	case blockID.IsPreConfirmed():
		pending, err := h.PendingData()
		if err != nil {
			return nil, rpccore.ErrBlockNotFound
		}

		if uint64(txIndex) >= pending.GetBlock().TransactionCount {
			return nil, rpccore.ErrInvalidTxIndex
		}

		adaptedTxn := AdaptTransaction(pending.GetBlock().Transactions[txIndex], includeProofFacts)
		return &adaptedTxn, nil
	case blockID.IsLatest():
		header, err := h.bcReader.HeadsHeader()
		if err != nil {
			return nil, rpccore.ErrBlockNotFound
		}
		blockNumber = header.Number
	case blockID.IsHash():
		blockNumber, err = h.bcReader.BlockNumberByHash(blockID.Hash())
	case blockID.IsNumber():
		blockNumber = blockID.Number()
	case blockID.IsL1Accepted():
		var l1Head core.L1Head
		l1Head, err = h.bcReader.L1Head()
		if err != nil {
			break
		}
		blockNumber = l1Head.BlockNumber
	default:
		panic("unknown block type id")
	}

	if err != nil {
		return nil, rpccore.ErrBlockNotFound
	}

	txn, err := h.bcReader.TransactionByBlockNumberAndIndex(blockNumber, uint64(txIndex))
	if err != nil {
		return nil, rpccore.ErrInvalidTxIndex
	}
	adaptedTxn := AdaptTransaction(txn, includeProofFacts)
	return &adaptedTxn, nil
}

// TransactionReceiptByHash returns the receipt of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L222
func (h *Handler) TransactionReceiptByHash(
	hash *felt.Felt,
) (*rpcv9.TransactionReceipt, *jsonrpc.Error) {
	adaptedReceipt, rpcErr := h.getPendingTransactionReceipt(hash)
	if rpcErr == nil {
		return adaptedReceipt, nil
	}

	blockNumber, idx, err := h.bcReader.BlockNumberAndIndexByTxHash((*felt.TransactionHash)(hash))
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return nil, rpccore.ErrTxnHashNotFound
	}

	txn, err := h.bcReader.TransactionByBlockNumberAndIndex(blockNumber, idx)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return nil, rpccore.ErrTxnHashNotFound
	}

	receipt, blockHash, err := h.bcReader.ReceiptByBlockNumberAndIndex(blockNumber, idx)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, rpccore.ErrInternal.CloneWithData(err)
		}
		return nil, rpccore.ErrTxnHashNotFound
	}

	l1H, jsonErr := h.l1Head()
	if jsonErr != nil {
		return nil, jsonErr
	}

	status := rpcv9.TxnAcceptedOnL2
	if isL1Verified(blockNumber, l1H) {
		status = rpcv9.TxnAcceptedOnL1
	}

	return rpcv9.AdaptReceiptWithBlockInfo(
		&receipt,
		txn,
		status,
		blockHash,
		blockNumber,
		false,
	), nil
}

// getPendingTransactionReceipt searches for a transaction receipt in the pending data.
// Returns the receipt if found, otherwise returns `rpccore.ErrTxnHashNotFound`.
func (h *Handler) getPendingTransactionReceipt(
	hash *felt.Felt,
) (*rpcv9.TransactionReceipt, *jsonrpc.Error) {
	pending, err := h.PendingData()
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	receipt, parentHash, blockNumber, err := pending.ReceiptByHash(hash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	txn, err := pending.TransactionByHash(hash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	status := rpcv9.TxnPreConfirmed
	isPreLatest := false
	if parentHash != nil {
		// pre-latest block or pending block
		status = rpcv9.TxnAcceptedOnL2
		// If pending data is pre_confirmed receipt is coming from pre_latest
		isPreLatest = pending.Variant() == core.PreConfirmedBlockVariant
	}
	return rpcv9.AdaptReceiptWithBlockInfo(
		receipt,
		txn,
		status,
		nil,
		blockNumber,
		isPreLatest,
	), nil
}

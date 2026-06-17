package rpcv10

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"go.uber.org/zap"
)

var (
	gzPool = sync.Pool{
		New: func() any {
			w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
			return w
		},
	}
	bufPool = sync.Pool{
		New: func() any { return new(bytes.Buffer) },
	}
)

// AdaptTransaction adapts a core.Transaction to a local *Transaction.
// It's a wrapper around AdaptCoreTransaction that allows to exclude proof facts
// from the transaction. If includeProofFacts is false, the proof facts are set to nil,
// otherwise they're returned as is.
func AdaptTransaction(coreTx core.Transaction, includeProofFacts bool) Transaction {
	tx := *AdaptCoreTransaction(coreTx)

	if _, ok := coreTx.(*core.InvokeTransaction); ok {
		if !includeProofFacts {
			tx.ProofFacts = nil
		} else if tx.ProofFacts == nil {
			// When proof facts are requested for INVOKE V3 transactions,
			// but they are not available, return an empty array
			emptyProofFacts := []felt.Felt{}
			tx.ProofFacts = &emptyProofFacts
		}
	}

	return tx
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
		var userTxn core.Transaction
		res, userTxn, err = h.addToMempool(ctx, tx)
		if err != nil {
			return AddTxResponse{}, err
		}

		if h.receivedTransactionFeed != nil {
			h.receivedTransactionFeed.Send(userTxn)
		}
	} else {
		res, err = h.pushToFeederGateway(ctx, tx)
		if err != nil {
			return AddTxResponse{}, err
		}

		if h.receivedTransactionFeed != nil {
			adaptedTxn, _, aErr := adaptAndCompileBroadcastedTxToCore(
				ctx, h.compiler, tx, h.bcReader.Network(),
			)
			if aErr != nil {
				// Log error but don't fail the transaction submission
				h.logger.Warn("Failed to adapt transaction for received feed", zap.Error(aErr))
			} else {
				h.receivedTransactionFeed.Send(adaptedTxn)
			}
		}
	}

	if h.submittedTransactionsCache != nil {
		h.submittedTransactionsCache.Add((*felt.Felt)(&res.TransactionHash))
	}

	return res, nil
}

func adaptAndCompileBroadcastedTxToCore(
	ctx context.Context,
	compiler compiler.Compiler,
	tx *BroadcastedTransaction,
	network *networks.Network,
) (core.Transaction, core.ClassDefinition, error) {
	var classHash *felt.Felt
	var sierraClass core.ClassDefinition

	if tx.Type == TxnDeclare {
		class := adaptContractClassToStarknet(tx.ContractClass)
		compiledClass, err := compiler.Compile(ctx, &class)
		if err != nil {
			return nil, nil, err
		}
		sierraClass, err = sn2core.AdaptSierraClass(&class, compiledClass)
		if err != nil {
			return nil, nil, err
		}

		tempClassHash, err := sierraClass.Hash()
		if err != nil {
			return nil, nil, err
		}
		classHash = &tempClassHash
	}
	coreTx, err := AdaptBroadcastedTransactionToCore(ctx, tx, classHash, network)
	return coreTx, sierraClass, err
}

func (h *Handler) addToMempool(
	ctx context.Context,
	tx *BroadcastedTransaction,
) (AddTxResponse, core.Transaction, *jsonrpc.Error) {
	userTxn, userClass, err := adaptAndCompileBroadcastedTxToCore(
		ctx, h.compiler, tx, h.bcReader.Network(),
	)
	if err != nil {
		return AddTxResponse{}, nil, rpccore.ErrInternal.CloneWithData(err.Error())
	}

	if err = h.memPool.Push(ctx, &mempool.BroadcastedTransaction{
		Transaction:   userTxn,
		DeclaredClass: userClass,
		Proof:         tx.Proof,
	}); err != nil {
		return AddTxResponse{}, nil, rpccore.ErrInternal.CloneWithData(err.Error())
	}
	userTxnHash := felt.TransactionHash(*userTxn.Hash())
	res := AddTxResponse{TransactionHash: userTxnHash}
	switch tx.Type {
	case TxnDeployAccount:
		contractAddress := core.ContractAddress(
			&felt.Zero,
			tx.ClassHash,
			tx.ContractAddressSalt,
			*tx.ConstructorCallData,
		)
		res.ContractAddress = (*felt.Address)(&contractAddress)
	case TxnDeclare:
		// Class hash was already computed in adaptAndCompileBroadcastedTxToCore.
		res.ClassHash = (*felt.ClassHash)(userTxn.(*core.DeclareTransaction).ClassHash)
	}
	return res, userTxn, nil
}

func (h *Handler) pushToFeederGateway(
	ctx context.Context,
	tx *BroadcastedTransaction,
) (AddTxResponse, *jsonrpc.Error) {
	classPayload, err := ContractClassToGatewayPayload(tx.ContractClass)
	if err != nil {
		return AddTxResponse{}, rpccore.ErrInternal.CloneWithData(
			fmt.Sprintf("failed to get contract class payload: %v", err),
		)
	}

	payload := AddTxGatewayPayload{
		Transaction:   AdaptBroadcastedTransactionToFeeder(tx),
		ContractClass: classPayload,
		Proof:         tx.Proof,
	}

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
		jsonErr := MakeJSONErrorFromGatewayError(err)
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

// ContractClassToGatewayPayload returns the contract class payload in the format
// expected by the gateway.
func ContractClassToGatewayPayload(class *ContractClass) ([]byte, error) {
	if class == nil {
		return []byte{}, nil
	}

	sierraBuf := bufPool.Get().(*bytes.Buffer)
	sierraBuf.Reset()
	defer bufPool.Put(sierraBuf)

	b64 := base64.NewEncoder(base64.StdEncoding, sierraBuf)
	gz := gzPool.Get().(*gzip.Writer)
	gz.Reset(b64)
	defer gzPool.Put(gz)

	enc := json.NewEncoder(gz)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(class.SierraProgram); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	if err := b64.Close(); err != nil {
		return nil, err
	}

	temp := struct {
		SierraProgram        string                   `json:"sierra_program"`
		ContractClassVersion string                   `json:"contract_class_version"`
		EntryPoints          ContractClassEntryPoints `json:"entry_points_by_type"`
		ABI                  string                   `json:"abi,omitempty"`
	}{
		SierraProgram:        sierraBuf.String(),
		ContractClassVersion: class.ContractClassVersion,
		EntryPoints:          class.EntryPoints,
		ABI:                  class.ABI,
	}
	return json.Marshal(temp)
}

func (h *Handler) TransactionStatus(
	ctx context.Context,
	hash *felt.Felt,
) (TransactionStatus, *jsonrpc.Error) {
	receipt, txErr := h.TransactionReceiptByHash(hash)
	switch txErr {
	case nil:
		return TransactionStatus{
			Finality:      TxnStatus(receipt.FinalityStatus),
			Execution:     receipt.ExecutionStatus,
			FailureReason: receipt.RevertReason,
		}, nil
	case rpccore.ErrTxnHashNotFound:
		// Search pre-confirmed block for 'CANDIDATE' status
		var txStatus *starknet.TransactionStatus
		var err error
		preConfirmedB, err := h.syncReader.PreConfirmed()

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

			txStatus, err = h.feederClient.TransactionStatus(ctx, hash)
			if err != nil {
				return TransactionStatus{}, jsonrpc.Err(jsonrpc.InternalError, err.Error())
			}

			if txStatus.FinalityStatus == starknet.NotReceived && h.submittedTransactionsCache != nil {
				if h.submittedTransactionsCache.Contains(hash) {
					txStatus.FinalityStatus = starknet.Received
				}
			}
		}

		status, err := AdaptTransactionStatus(txStatus)
		if err != nil {
			if !errors.Is(err, ErrTransactionNotFound) {
				h.logger.Error("Failed to adapt transaction status", zap.Error(err))
			}
			return TransactionStatus{}, rpccore.ErrTxnHashNotFound
		}
		return status, nil
	}
	return TransactionStatus{}, txErr
}

/****************************************************
		Transaction Handlers
*****************************************************/

// TransactionByHash returns the details of a transaction identified by the given hash.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/0bf403bfafbfbe0eaa52103a9c7df545bec8f73b/api/starknet_api_openrpc.json#L315
func (h *Handler) TransactionByHash(
	hash *felt.Felt,
	responseFlags ResponseFlags,
) (*Transaction, *jsonrpc.Error) {
	// Check pre-confirmed data
	if preConfirmed, err := h.syncReader.PreConfirmed(); err == nil {
		if txn, err := preConfirmed.TransactionByHash(hash); err == nil {
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

func (h *Handler) TransactionByBlockIDAndIndex(
	blockID *BlockID, txIndex int, responseFlags ResponseFlags,
) (*Transaction, *jsonrpc.Error) {
	includeProofFacts := responseFlags.IncludeProofFacts

	if txIndex < 0 {
		return nil, rpccore.ErrInvalidTxIndex
	}

	var blockNumber uint64
	var err error
	switch {
	case blockID.IsPreConfirmed():
		preConfirmed, err := h.syncReader.PreConfirmed()
		if err != nil {
			return nil, rpccore.ErrBlockNotFound
		}

		if uint64(txIndex) >= preConfirmed.GetBlock().TransactionCount {
			return nil, rpccore.ErrInvalidTxIndex
		}

		adaptedTxn := AdaptTransaction(preConfirmed.GetBlock().Transactions[txIndex], includeProofFacts)
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
		blockNumber, err = h.l1AcceptedBlockNumber()
		if err != nil {
			break
		}
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
) (*TransactionReceipt, *jsonrpc.Error) {
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

	status := TxnAcceptedOnL2
	if isL1Verified(blockNumber, l1H) {
		status = TxnAcceptedOnL1
	}

	return AdaptReceiptWithBlockInfo(
		&receipt,
		txn,
		status,
		blockHash,
		blockNumber,
	), nil
}

// getPendingTransactionReceipt searches for a transaction receipt in the pre_confirmed block.
// Returns the receipt if found, otherwise returns `rpccore.ErrTxnHashNotFound`.
func (h *Handler) getPendingTransactionReceipt(
	hash *felt.Felt,
) (*TransactionReceipt, *jsonrpc.Error) {
	preConfirmed, err := h.syncReader.PreConfirmed()
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	receipt, _, blockNumber, err := preConfirmed.ReceiptByHash(hash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	txn, err := preConfirmed.TransactionByHash(hash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	return AdaptReceiptWithBlockInfo(
		receipt,
		txn,
		TxnPreConfirmed,
		nil,
		blockNumber,
	), nil
}

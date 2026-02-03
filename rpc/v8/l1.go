package rpcv8

import (
	"context"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/l1/types"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"golang.org/x/crypto/sha3"
)

var logMsgToL2SigHash = types.UnsafeFromString[types.L1Hash]("0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b")

type logMessageToL2 struct {
	FromAddress *types.L1Address
	ToAddress   *big.Int
	Nonce       *big.Int
	Selector    *big.Int
	Payload     []*big.Int
	Fee         *big.Int
}

func (l *logMessageToL2) hashMessage() types.L1Hash {
	hash := sha3.NewLegacyKeccak256()

	writeUint256 := func(value *big.Int) {
		bytes := make([]byte, 32)
		value.FillBytes(bytes)
		hash.Write(bytes)
	}

	fromBytes := l.FromAddress.Bytes()
	hash.Write(fromBytes[:])
	writeUint256(l.ToAddress)
	writeUint256(l.Nonce)
	writeUint256(l.Selector)
	writeUint256(big.NewInt(int64(len(l.Payload))))

	for _, elem := range l.Payload {
		writeUint256(elem)
	}

	eventHash := hash.Sum(nil)
	return types.FromBytes[types.L1Hash](eventHash)
}

type MsgStatus struct {
	L1HandlerHash  *felt.Felt `json:"transaction_hash"`
	FinalityStatus TxnStatus  `json:"finality_status"`
	FailureReason  string     `json:"failure_reason,omitempty"`
}

func (h *Handler) GetMessageStatus(ctx context.Context, l1TxnHash *types.L1Hash) ([]MsgStatus, *jsonrpc.Error) {
	// l1 txn hash -> (l1 handler) msg hashes
	msgHashes, rpcErr := h.messageToL2Logs(ctx, l1TxnHash)
	if rpcErr != nil {
		return nil, rpcErr
	}
	// (l1 handler) msg hashes -> l1 handler txn hashes
	results := make([]MsgStatus, len(msgHashes))
	for i, msgHash := range msgHashes {
		hash, err := h.bcReader.L1HandlerTxnHash(msgHash)
		if err != nil {
			return nil, jsonrpc.Err(
				jsonrpc.InternalError,
				fmt.Errorf("failed to retrieve L1 handler txn %v",
					err),
			)
		}
		status, rpcErr := h.TransactionStatus(ctx, hash)
		if rpcErr != nil {
			return nil, rpcErr
		}
		results[i] = MsgStatus{
			L1HandlerHash:  &hash,
			FinalityStatus: status.Finality,
			FailureReason:  status.FailureReason,
		}
	}
	return results, nil
}

func (h *Handler) messageToL2Logs(ctx context.Context, txHash *types.L1Hash) ([]*types.L1Hash, *jsonrpc.Error) {
	if h.l1Client == nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, "L1 client not found")
	}

	receipt, err := h.l1Client.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	messageHashes := make([]*types.L1Hash, 0, len(receipt.Logs))
	for _, vLog := range receipt.Logs {
		topic0Hash, err := types.FromString[types.L1Hash](vLog.Topics[0].Hex())
		if err != nil {
			return nil, jsonrpc.Err(rpccore.ErrInternal.Code, fmt.Errorf("failed to parse topic 0 hash %v", err))
		}
		if !types.Equal(&topic0Hash, &logMsgToL2SigHash) {
			continue
		}
		var event logMessageToL2
		err = h.coreContractABI.UnpackIntoInterface(&event, "LogMessageToL2", vLog.Data)
		if err != nil {
			return nil, jsonrpc.Err(rpccore.ErrInternal.Code, fmt.Errorf("failed to unpack log %v", err))
		}
		// Extract indexed fields from topics
		fromAddress, err := types.FromString[types.L1Address](vLog.Topics[1].Hex())
		if err != nil {
			return nil, jsonrpc.Err(rpccore.ErrInternal.Code, fmt.Errorf("failed to parse from address %v", err))
		}
		event.ToAddress = new(big.Int).SetBytes(vLog.Topics[2].Bytes())
		selector := new(big.Int).SetBytes(vLog.Topics[3].Bytes())
		event.FromAddress = &fromAddress
		event.Selector = selector
		eventHash := event.hashMessage()
		messageHashes = append(messageHashes, &eventHash)
	}
	return messageHashes, nil
}

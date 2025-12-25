package rpcv8

import (
	"context"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
)

var logMsgToL2SigHash = common.HexToHash("0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b")

type logMessageToL2 struct {
	FromAddress *common.Address
	ToAddress   *big.Int
	Nonce       *big.Int
	Selector    *big.Int
	Payload     []*big.Int
	Fee         *big.Int
}

func (l *logMessageToL2) hashMessage() *common.Hash {
	hash := sha3.NewLegacyKeccak256()

	writeUint256 := func(value *big.Int) {
		bytes := make([]byte, 32)
		value.FillBytes(bytes)
		hash.Write(bytes)
	}

	// Pad FromAddress to 32 bytes
	hash.Write(make([]byte, 12)) //nolint:mnd
	hash.Write(l.FromAddress.Bytes())
	writeUint256(l.ToAddress)
	writeUint256(l.Nonce)
	writeUint256(l.Selector)
	writeUint256(big.NewInt(int64(len(l.Payload))))

	for _, elem := range l.Payload {
		writeUint256(elem)
	}

	tmp := common.BytesToHash(hash.Sum(nil))
	return &tmp
}

type MsgStatus struct {
	L1HandlerHash  *felt.Felt `json:"transaction_hash"`
	FinalityStatus TxnStatus  `json:"finality_status"`
	FailureReason  string     `json:"failure_reason,omitempty"`
}

func (h *Handler) GetMessageStatus(ctx context.Context, l1TxnHash *common.Hash) ([]MsgStatus, *jsonrpc.Error) {
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
			return nil, jsonrpc.Err(jsonrpc.InternalError, fmt.Errorf("failed to retrieve L1 handler txn %v", err))
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

func (h *Handler) messageToL2Logs(ctx context.Context, txHash *common.Hash) ([]*common.Hash, *jsonrpc.Error) {
	if h.l1Client == nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, "L1 client not found")
	}

	receipt, err := h.l1Client.TransactionReceipt(ctx, *txHash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	messageHashes := make([]*common.Hash, 0, len(receipt.Logs))
	for _, vLog := range receipt.Logs {
		if common.HexToHash(vLog.Topics[0].Hex()).Cmp(logMsgToL2SigHash) != 0 {
			continue
		}
		var event logMessageToL2
		err = h.coreContractABI.UnpackIntoInterface(&event, "LogMessageToL2", vLog.Data)
		if err != nil {
			return nil, jsonrpc.Err(rpccore.ErrInternal.Code, fmt.Errorf("failed to unpack log %v", err))
		}
		// Extract indexed fields from topics
		fromAddress := common.HexToAddress(vLog.Topics[1].Hex())
		event.ToAddress = new(big.Int).SetBytes(vLog.Topics[2].Bytes())
		selector := new(big.Int).SetBytes(vLog.Topics[3].Bytes())
		event.FromAddress = &fromAddress
		event.Selector = selector
		messageHashes = append(messageHashes, event.hashMessage())
	}
	return messageHashes, nil
}

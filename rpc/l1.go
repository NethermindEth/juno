package rpc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
)

var ErrL1ClientNotFound = jsonrpc.Err(jsonrpc.InternalError, fmt.Errorf("L1 client not found, cannot serve starknet_getMessage"))

type LogMessageToL2 struct {
	FromAddress *common.Address
	ToAddress   *common.Address
	Nonce       *big.Int
	Selector    *big.Int
	Payload     []*big.Int
	Fee         *big.Int
}

// HashMessage calculates the message hash following the Keccak256 hash method
func (l *LogMessageToL2) HashMessage() *common.Hash {
	hash := sha3.NewLegacyKeccak256()

	// Padding for Ethereum address to 32 bytes
	hash.Write(make([]byte, 12)) //nolint:mnd
	hash.Write(l.FromAddress.Bytes())
	hash.Write(l.ToAddress.Bytes())
	hash.Write(l.Nonce.Bytes())
	hash.Write(l.Selector.Bytes())

	// Padding for payload length (u64)
	hash.Write(make([]byte, 24))     //nolint:mnd
	payloadLength := make([]byte, 8) //nolint:mnd
	big.NewInt(int64(len(l.Payload))).FillBytes(payloadLength)
	hash.Write(payloadLength)

	for _, elem := range l.Payload {
		hash.Write(elem.Bytes())
	}
	tmp := common.BytesToHash(hash.Sum(nil))
	return &tmp
}

type MsgStatus struct {
	L1HandlerHash  *felt.Felt
	FinalityStatus TxnStatus
	FailureReason  string
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
		status, rpcErr := h.TransactionStatus(ctx, *hash)
		if rpcErr != nil {
			return nil, rpcErr
		}
		results[i] = MsgStatus{
			L1HandlerHash:  hash,
			FinalityStatus: status.Finality,
			FailureReason:  status.FailureReason,
		}
	}
	return results, nil
}

func (h *Handler) messageToL2Logs(ctx context.Context, txHash *common.Hash) ([]*common.Hash, *jsonrpc.Error) {
	if h.ethClient == nil {
		return nil, ErrL1ClientNotFound
	}

	receipt, err := h.ethClient.TransactionReceipt(ctx, *txHash)
	if err != nil {
		return nil, ErrTxnHashNotFound
	}
	logMsgToL2SigHash := common.HexToHash("0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b")

	var messageHashes []*common.Hash
	for _, vLog := range receipt.Logs {
		if common.HexToHash(vLog.Topics[0].Hex()).Cmp(logMsgToL2SigHash) != 0 {
			continue
		}
		var event LogMessageToL2
		err = h.coreContractABI.UnpackIntoInterface(&event, "LogMessageToL2", vLog.Data)
		if err != nil {
			return nil, jsonrpc.Err(ErrInternal.Code, fmt.Errorf("failed to unpack log %v", err))
		}
		// Extract indexed fields from topics
		fromAddress := common.HexToAddress(vLog.Topics[1].Hex())
		toAddress := common.HexToAddress(vLog.Topics[2].Hex())
		selector := new(big.Int).SetBytes(vLog.Topics[3].Bytes())
		event.FromAddress = &fromAddress
		event.ToAddress = &toAddress
		event.Selector = selector
		messageHashes = append(messageHashes, event.HashMessage())
	}
	return messageHashes, nil
}

package rpcv8

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/abi"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"golang.org/x/crypto/sha3"
)

var logMsgToL2SigHash = eth.HashFromString(
	"0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b",
)

type logMessageToL2 struct {
	FromAddress eth.Address
	ToAddress   abi.Word
	Nonce       abi.Word
	Selector    abi.Word
	Payload     []abi.Word
	Fee         abi.Word
}

func (l *logMessageToL2) hashMessage() *eth.Hash {
	hash := sha3.NewLegacyKeccak256()

	// Pad FromAddress to 32 bytes
	hash.Write(make([]byte, 12)) //nolint:mnd
	hash.Write(l.FromAddress.Bytes())

	hash.Write(l.ToAddress[:])
	hash.Write(l.Nonce[:])
	hash.Write(l.Selector[:])

	var lenWord abi.Word
	binary.BigEndian.PutUint64(lenWord[24:], uint64(len(l.Payload))) //nolint:mnd // 32 - 8 = 24
	hash.Write(lenWord[:])

	for i := range l.Payload {
		hash.Write(l.Payload[i][:])
	}

	tmp := eth.HashFromBytes(hash.Sum(nil))
	return &tmp
}

type MsgStatus struct {
	L1HandlerHash  *felt.Felt `json:"transaction_hash"`
	FinalityStatus TxnStatus  `json:"finality_status"`
	FailureReason  string     `json:"failure_reason,omitempty"`
}

func (h *Handler) GetMessageStatus(
	ctx context.Context, l1TxnHash *eth.Hash,
) ([]MsgStatus, *jsonrpc.Error) {
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
				fmt.Sprintf(
					"failed to retrieve L1 handler txn hash. msgHash %s, err: %v",
					msgHash.Hex(),
					err,
				),
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

func (h *Handler) messageToL2Logs(
	ctx context.Context, txHash *eth.Hash,
) ([]*eth.Hash, *jsonrpc.Error) {
	if h.l1Client == nil {
		return nil, jsonrpc.Err(jsonrpc.InternalError, "L1 client not found")
	}

	receipt, err := h.l1Client.TransactionReceipt(ctx, *txHash)
	if err != nil {
		return nil, rpccore.ErrTxnHashNotFound
	}

	messageHashes := make([]*eth.Hash, 0, len(receipt.Logs))
	for i, vLog := range receipt.Logs {
		if vLog.Topics[0].Cmp(logMsgToL2SigHash) != 0 {
			continue
		}
		payload, nonce, fee, err := abi.UnpackLogMessageToL2(vLog.Data)
		if err != nil {
			return nil, jsonrpc.Err(
				jsonrpc.InternalError,
				fmt.Sprintf(
					"failed to unpack log, l1 txn hash %s, logIndex %d err: %v",
					txHash.Hex(),
					i,
					err,
				),
			)
		}
		event := logMessageToL2{
			FromAddress: eth.AddressFromBytes(vLog.Topics[1].Bytes()),
			ToAddress:   abi.Word(vLog.Topics[2]),
			Selector:    abi.Word(vLog.Topics[3]),
			Payload:     payload,
			Nonce:       nonce,
			Fee:         fee,
		}
		messageHashes = append(messageHashes, event.hashMessage())
	}
	return messageHashes, nil
}

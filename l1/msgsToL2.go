package l1

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/NethermindEth/juno/l1/contract"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
)

type MsgAndTxnHash struct {
	L1TxnHash *common.Hash
	MsgHash   *common.Hash
}

type LogMessageToL2 struct {
	FromAddress *common.Address
	ToAddress   *common.Address
	Nonce       *big.Int
	Selector    *big.Int
	Payload     []*big.Int
	Fee         *big.Int
}

// HashMessage calculates the message hash following the Keccak256 hash method
func (log *LogMessageToL2) HashMessage() *common.Hash {
	hash := sha3.NewLegacyKeccak256()

	// Padding for Ethereum address to 32 bytes
	hash.Write(make([]byte, 12))
	hash.Write(log.FromAddress.Bytes())
	hash.Write(log.ToAddress.Bytes())
	hash.Write(log.Nonce.Bytes())
	hash.Write(log.Selector.Bytes())

	// Padding for payload length (u64)
	hash.Write(make([]byte, 24))
	payloadLength := make([]byte, 8)
	big.NewInt(int64(len(log.Payload))).FillBytes(payloadLength)
	hash.Write(payloadLength)

	for _, elem := range log.Payload {
		hash.Write(elem.Bytes())
	}
	tmp := common.BytesToHash(hash.Sum(nil))
	return &tmp
}

// MessageToL2Logs pulls the "LogMessageToL2" event from the transaction and returns the message hash and txn hash
func (es *EthSubscriber) MessageToL2Logs(txHash common.Hash) ([]MsgAndTxnHash, error) {
	// Todo: push to consts
	logMsgToL2SigHash := common.HexToHash("0xdb80dd488acf86d17c747445b0eabb5d57c541d3bd7b6b87af987858e5066b2b")
	contractABI, err := abi.JSON(strings.NewReader(contract.StarknetMetaData.ABI))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v", err)
	}

	receipt, err := es.ethClient.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction receipt: %v", err)
	}

	var results []MsgAndTxnHash
	for _, vLog := range receipt.Logs {
		if common.HexToHash(vLog.Topics[0].Hex()).Cmp(logMsgToL2SigHash) != 0 {
			continue
		}
		var event LogMessageToL2
		err = contractABI.UnpackIntoInterface(&event, "LogMessageToL2", vLog.Data)
		if err != nil {
			log.Fatalf("Failed to unpack log: %v", err)
		}
		// Extract indexed fields from topics
		fromAddress := common.HexToAddress(vLog.Topics[1].Hex())
		toAddress := common.HexToAddress(vLog.Topics[2].Hex())
		selector := new(big.Int).SetBytes(vLog.Topics[3].Bytes())
		event.FromAddress = &fromAddress
		event.ToAddress = &toAddress
		event.Selector = selector
		results = append(results, MsgAndTxnHash{L1TxnHash: &vLog.TxHash, MsgHash: event.HashMessage()})
	}
	return results, nil
}

package db

import (
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
)

func PeerKey(peerID []byte) []byte {
	return Peer.Key(peerID)
}

func ContractClassHashKey(addr *felt.Felt) []byte {
	return ContractClassHash.Key(addr.Marshal())
}

func ContractStorageKey(addr *felt.Felt, key []byte) []byte {
	return ContractStorage.Key(addr.Marshal(), key)
}

func ClassKey(classHash *felt.Felt) []byte {
	return Class.Key(classHash.Marshal())
}

func ContractNonceKey(addr *felt.Felt) []byte {
	return ContractNonce.Key(addr.Marshal())
}

func BlockHeaderNumbersByHashKey(hash *felt.Felt) []byte {
	return BlockHeaderNumbersByHash.Key(hash.Marshal())
}

func BlockHeaderByNumberKey(blockNum uint64) []byte {
	b := uint64ToBytes(blockNum)
	return BlockHeadersByNumber.Key(b[:])
}

func TxBlockNumIndexByHashKey(hash *felt.Felt) []byte {
	return TransactionBlockNumbersAndIndicesByHash.Key(hash.Marshal())
}

const BlockNumIndexKeySize = 16

type BlockNumIndexKey struct {
	Number uint64
	Index  uint64
}

func (b *BlockNumIndexKey) MarshalBinary() []byte {
	data := make([]byte, BlockNumIndexKeySize)
	binary.BigEndian.PutUint64(data[0:8], b.Number)
	binary.BigEndian.PutUint64(data[8:16], b.Index)
	return data
}

func (b *BlockNumIndexKey) UnmarshalBinary(data []byte) error {
	if len(data) < BlockNumIndexKeySize {
		return errors.New("data is too short to unmarshal block number and index")
	}
	b.Number = binary.BigEndian.Uint64(data[0:8])
	b.Index = binary.BigEndian.Uint64(data[8:16])
	return nil
}

func TxByBlockNumIndexKey(num, index uint64) []byte {
	key := &BlockNumIndexKey{Number: num, Index: index}
	return TransactionsByBlockNumberAndIndex.Key(key.MarshalBinary())
}

func TxByBlockNumIndexKeyBytes(key []byte) []byte {
	return TransactionsByBlockNumberAndIndex.Key(key)
}

func ReceiptByBlockNumIndexKey(num, index uint64) []byte {
	key := &BlockNumIndexKey{Number: num, Index: index}
	return ReceiptsByBlockNumberAndIndex.Key(key.MarshalBinary())
}

func ReceiptByBlockNumIndexKeyBytes(key []byte) []byte {
	return ReceiptsByBlockNumberAndIndex.Key(key)
}

func StateUpdateByBlockNumKey(num uint64) []byte {
	b := uint64ToBytes(num)
	return StateUpdatesByBlockNumber.Key(b[:])
}

func ContractStorageHistoryKey(addr, loc *felt.Felt) []byte {
	return ContractStorageHistory.Key(addr.Marshal(), loc.Marshal())
}

func ContractNonceHistoryKey(addr *felt.Felt) []byte {
	return ContractNonceHistory.Key(addr.Marshal())
}

func ContractClassHashHistoryKey(addr *felt.Felt) []byte {
	return ContractClassHashHistory.Key(addr.Marshal())
}

func ContractDeploymentHeightKey(addr *felt.Felt) []byte {
	return ContractDeploymentHeight.Key(addr.Marshal())
}

func BlockCommitmentsKey(blockNum uint64) []byte {
	b := uint64ToBytes(blockNum)
	return BlockCommitments.Key(b[:])
}

func L1HandlerTxnHashByMsgHashKey(msgHash []byte) []byte {
	return L1HandlerTxnHashByMsgHash.Key(msgHash)
}

func MempoolNodeKey(txnHash *felt.Felt) []byte {
	return MempoolNode.Key(txnHash.Marshal())
}

func ContractKey(addr *felt.Felt) []byte {
	return Contract.Key(addr.Marshal())
}

func ContractHistoryNonceKey(addr *felt.Felt, blockNum uint64) []byte {
	b := uint64ToBytes(blockNum)
	return ContractNonceHistory.Key(addr.Marshal(), b[:])
}

func ContractHistoryClassHashKey(addr *felt.Felt, blockNum uint64) []byte {
	b := uint64ToBytes(blockNum)
	return ContractClassHashHistory.Key(addr.Marshal(), b[:])
}

func ContractHistoryStorageKey(addr, key *felt.Felt, blockNum uint64) []byte {
	b := uint64ToBytes(blockNum)
	return ContractStorageHistory.Key(addr.Marshal(), key.Marshal(), b[:])
}

func StateIDKey(root *felt.Felt) []byte {
	return StateID.Key(root.Marshal())
}

func uint64ToBytes(num uint64) [8]byte {
	var numBytes [8]byte
	binary.BigEndian.PutUint64(numBytes[:], num)
	return numBytes
}

func StateHashToTrieRootsKey(stateCommitment *felt.Felt) []byte {
	return StateHashToTrieRoots.Key(stateCommitment.Marshal())
}
